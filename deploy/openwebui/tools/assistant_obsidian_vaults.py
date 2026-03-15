"""
title: Assistant Obsidian Vaults
author: personal-ai-assistant
author_url: https://github.com/kirillkom/personal-ai-assistant
description: Manage Obsidian vaults for automatic synchronization with the assistant backend.
required_open_webui_version: 0.6.0
version: 0.1.0
"""

import hashlib
import json
import os
import re
import time
from typing import Any, Dict, List, Tuple

import requests


CONFIG_PATH = os.environ.get(
    "ASSISTANT_OBSIDIAN_CONFIG_PATH",
    "/app/backend/data/assistant/obsidian_vaults.json",
)
STATE_DIR = os.environ.get(
    "ASSISTANT_OBSIDIAN_STATE_DIR",
    "/app/backend/data/assistant/obsidian_state",
)
VAULTS_ROOT = os.environ.get("ASSISTANT_OBSIDIAN_VAULTS_ROOT", "/vaults")
ASSISTANT_API_URL = os.environ.get("ASSISTANT_API_URL", "http://api:8080").rstrip("/")
ASSISTANT_SYNC_TIMEOUT = int(os.environ.get("ASSISTANT_OBSIDIAN_SYNC_TIMEOUT_SECONDS", "120"))
ASSISTANT_SYNC_POLL_INTERVAL = float(os.environ.get("ASSISTANT_OBSIDIAN_SYNC_POLL_SECONDS", "2"))


def _default_config() -> Dict[str, Any]:
    default_interval = int(os.environ.get("ASSISTANT_OBSIDIAN_DEFAULT_INTERVAL_MINUTES", "15"))
    return {"vaults": [], "default_interval_minutes": default_interval}


def _load_config() -> Dict[str, Any]:
    if not os.path.isfile(CONFIG_PATH):
        return _default_config()
    try:
        with open(CONFIG_PATH, "r", encoding="utf-8") as f:
            data = json.load(f)
    except Exception:
        return _default_config()
    if not isinstance(data, dict):
        return _default_config()
    if "vaults" not in data or not isinstance(data["vaults"], list):
        data["vaults"] = []
    if "default_interval_minutes" not in data:
        data["default_interval_minutes"] = _default_config()["default_interval_minutes"]
    return data


def _save_config(cfg: Dict[str, Any]) -> None:
    os.makedirs(os.path.dirname(CONFIG_PATH), exist_ok=True)
    with open(CONFIG_PATH, "w", encoding="utf-8") as f:
        json.dump(cfg, f, ensure_ascii=False, indent=2)


def _slugify(name: str) -> str:
    name = name.strip().lower()
    name = re.sub(r"[^a-z0-9_.-]+", "_", name)
    return name or "vault"


def _normalize_path(path: str) -> str:
    path = (path or "").strip()
    if not path:
        return ""
    if os.path.isabs(path):
        return path
    return os.path.join(VAULTS_ROOT, path)


def _load_meta(vault_id: str) -> Dict[str, Any]:
    meta_path = os.path.join(STATE_DIR, f"{vault_id}.meta.json")
    if not os.path.isfile(meta_path):
        return {}
    try:
        with open(meta_path, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return {}


def _state_path(vault_id: str) -> str:
    return os.path.join(STATE_DIR, f"{vault_id}.tsv")


def _load_state(vault_id: str) -> Dict[str, Tuple[str, str]]:
    state: Dict[str, Tuple[str, str]] = {}
    path = _state_path(vault_id)
    if not os.path.isfile(path):
        return state
    try:
        with open(path, "r", encoding="utf-8") as f:
            for line in f:
                parts = line.rstrip("\n").split("\t")
                if len(parts) < 2:
                    continue
                rel = parts[0]
                h = parts[1]
                doc_id = parts[2] if len(parts) > 2 else ""
                if rel:
                    state[rel] = (h, doc_id)
    except Exception:
        return state
    return state


def _write_state(vault_id: str, rows: List[Tuple[str, str, str]]) -> None:
    os.makedirs(STATE_DIR, exist_ok=True)
    path = _state_path(vault_id)
    with open(path, "w", encoding="utf-8") as f:
        for rel, h, doc_id in rows:
            f.write(f"{rel}\t{h}\t{doc_id}\n")


def _iter_markdown_files(root: str) -> List[str]:
    files: List[str] = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in (".obsidian", ".trash", ".git")]
        for filename in filenames:
            if filename.lower().endswith(".md"):
                files.append(os.path.join(dirpath, filename))
    return files


def _hash_file(path: str) -> str:
    hasher = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1024 * 1024)
            if not chunk:
                break
            hasher.update(chunk)
    return hasher.hexdigest()


def _upload_file(assistant_api_url: str, path: str) -> Dict[str, Any]:
    with open(path, "rb") as f:
        files = {"file": (os.path.basename(path), f, "text/markdown")}
        response = requests.post(f"{assistant_api_url}/v1/documents", files=files, timeout=ASSISTANT_SYNC_TIMEOUT)
    if response.status_code >= 300:
        raise RuntimeError(f"Assistant upload failed: {response.status_code} {response.text}")
    return response.json()


def _poll_ready(assistant_api_url: str, document_id: str, timeout_seconds: int) -> Dict[str, Any]:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        response = requests.get(f"{assistant_api_url}/v1/documents/{document_id}", timeout=30)
        if response.status_code >= 300:
            raise RuntimeError(
                f"Document status check failed for {document_id}: {response.status_code} {response.text}"
            )
        payload = response.json()
        status = payload.get("status")
        if status in ("ready", "failed"):
            return payload
        time.sleep(ASSISTANT_SYNC_POLL_INTERVAL)
    raise RuntimeError(f"Timeout waiting for document {document_id} to reach ready status")


class Tools:
    def obsidian_vault_list(self) -> str:
        """
        List configured Obsidian vaults and last sync status.
        """
        cfg = _load_config()
        now = int(time.time())
        out: List[Dict[str, Any]] = []
        for vault in cfg.get("vaults", []):
            entry = dict(vault)
            vault_id = entry.get("id") or _slugify(entry.get("name", "vault"))
            meta = _load_meta(vault_id)
            entry["id"] = vault_id
            entry["last_sync_epoch"] = meta.get("last_sync_epoch")
            entry["last_status"] = meta.get("last_status")
            entry["last_error"] = meta.get("last_error")
            if entry.get("last_sync_epoch"):
                entry["last_sync_age_seconds"] = max(0, now - int(entry["last_sync_epoch"]))
            out.append(entry)
        return json.dumps(
            {"vaults": out, "default_interval_minutes": cfg.get("default_interval_minutes")},
            ensure_ascii=False,
        )

    def obsidian_vault_upsert(
        self,
        name: str,
        path: str,
        enabled: bool = True,
        interval_minutes: int = None,
    ) -> str:
        """
        Create or update a vault entry.

        :param name: Unique vault name (used as identifier).
        :param path: Vault path. If relative, it is resolved under ASSISTANT_OBSIDIAN_VAULTS_ROOT.
        :param enabled: Whether auto sync is enabled.
        :param interval_minutes: Optional override for sync interval.
        """
        name = (name or "").strip()
        if not name:
            return json.dumps({"error": "name is required"}, ensure_ascii=False)
        resolved_path = _normalize_path(path)
        if not resolved_path:
            return json.dumps({"error": "path is required"}, ensure_ascii=False)
        if interval_minutes is not None and interval_minutes < 1:
            return json.dumps({"error": "interval_minutes must be >= 1"}, ensure_ascii=False)

        cfg = _load_config()
        vaults = cfg.get("vaults", [])
        vault_id = _slugify(name)
        updated = False
        for entry in vaults:
            if entry.get("name") == name or entry.get("id") == vault_id:
                entry.update(
                    {
                        "id": vault_id,
                        "name": name,
                        "path": resolved_path,
                        "enabled": bool(enabled),
                    }
                )
                if interval_minutes is not None:
                    entry["interval_minutes"] = int(interval_minutes)
                updated = True
                break

        if not updated:
            entry = {
                "id": vault_id,
                "name": name,
                "path": resolved_path,
                "enabled": bool(enabled),
            }
            if interval_minutes is not None:
                entry["interval_minutes"] = int(interval_minutes)
            vaults.append(entry)
        cfg["vaults"] = vaults
        _save_config(cfg)

        return json.dumps(
            {
                "status": "ok",
                "vault": {
                    "id": vault_id,
                    "name": name,
                    "path": resolved_path,
                    "enabled": bool(enabled),
                    "interval_minutes": interval_minutes,
                },
            },
            ensure_ascii=False,
        )

    def obsidian_vault_remove(self, name: str) -> str:
        """
        Remove a vault entry by name.
        """
        name = (name or "").strip()
        if not name:
            return json.dumps({"error": "name is required"}, ensure_ascii=False)

        cfg = _load_config()
        vaults = cfg.get("vaults", [])
        vault_id = _slugify(name)
        new_vaults = [v for v in vaults if v.get("name") != name and v.get("id") != vault_id]
        if len(new_vaults) == len(vaults):
            return json.dumps({"error": "vault not found"}, ensure_ascii=False)
        cfg["vaults"] = new_vaults
        _save_config(cfg)
        return json.dumps({"status": "ok", "removed": name}, ensure_ascii=False)

    def obsidian_vault_sync_now(self, name: str = "", wait_ready: bool = True) -> str:
        """
        Trigger immediate sync for a vault (or all enabled vaults when name is empty or 'all').
        """
        cfg = _load_config()
        vaults = cfg.get("vaults", [])
        if not vaults:
            return json.dumps({"error": "no vaults configured"}, ensure_ascii=False)

        name = (name or "").strip()
        targets: List[Dict[str, Any]] = []
        if name == "" or name.lower() == "all":
            targets = [v for v in vaults if v.get("enabled", True)]
        else:
            vault_id = _slugify(name)
            for v in vaults:
                if v.get("name") == name or v.get("id") == vault_id:
                    targets = [v]
                    break
        if not targets:
            return json.dumps({"error": "vault not found or disabled"}, ensure_ascii=False)

        results: List[Dict[str, Any]] = []
        for vault in targets:
            vault_name = vault.get("name") or "vault"
            vault_id = vault.get("id") or _slugify(vault_name)
            vault_path = _normalize_path(vault.get("path", ""))
            if not vault_path or not os.path.isdir(vault_path):
                results.append(
                    {
                        "name": vault_name,
                        "id": vault_id,
                        "status": "error",
                        "error": f"vault path not found: {vault_path}",
                    }
                )
                continue

            known = _load_state(vault_id)
            rows: List[Tuple[str, str, str]] = []
            uploaded = 0
            skipped = 0
            failed = 0
            errors: List[Dict[str, Any]] = []

            for path in _iter_markdown_files(vault_path):
                rel = os.path.relpath(path, vault_path)
                try:
                    h = _hash_file(path)
                except Exception as exc:
                    failed += 1
                    errors.append({"file": rel, "error": str(exc)})
                    prev = known.get(rel, ("", ""))
                    rows.append((rel, prev[0], prev[1]))
                    continue

                prev = known.get(rel)
                if prev and prev[0] == h:
                    rows.append((rel, prev[0], prev[1]))
                    skipped += 1
                    continue

                try:
                    upload = _upload_file(ASSISTANT_API_URL, path)
                    doc_id = upload.get("id", "")
                    if wait_ready and doc_id:
                        status_payload = _poll_ready(ASSISTANT_API_URL, doc_id, ASSISTANT_SYNC_TIMEOUT)
                        if status_payload.get("status") != "ready":
                            raise RuntimeError(status_payload.get("error") or "document processing failed")
                    rows.append((rel, h, doc_id))
                    uploaded += 1
                except Exception as exc:
                    failed += 1
                    errors.append({"file": rel, "error": str(exc)})
                    prev_doc = prev[1] if prev else ""
                    prev_hash = prev[0] if prev else ""
                    rows.append((rel, prev_hash, prev_doc))

            _write_state(vault_id, rows)
            results.append(
                {
                    "name": vault_name,
                    "id": vault_id,
                    "status": "ok",
                    "uploaded": uploaded,
                    "skipped": skipped,
                    "failed": failed,
                    "errors": errors,
                }
            )

        return json.dumps({"results": results}, ensure_ascii=False)
