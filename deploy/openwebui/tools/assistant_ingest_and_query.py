"""
title: Assistant Ingest And Query Tool
author: personal-ai-assistant
author_url: https://github.com/kirillkom/personal-ai-assistant
description: Uploads all attached files to Personal AI Assistant and runs RAG query.
required_open_webui_version: 0.6.0
version: 0.1.0
"""

import json
import os
import time
from typing import Any, Dict, List

import requests


class Tools:
    def ingest_and_query(
        self,
        question: str = "",
        __files__: List[Dict[str, Any]] = None,
        __request__=None,
    ) -> str:
        """
        Upload all attached files to the assistant backend and run RAG query.

        :param question: Optional question for RAG query. Fallback is summary prompt.
        """
        files = __files__ or []
        if not files:
            return json.dumps({"error": "No attached files were found"}, ensure_ascii=False)

        assistant_api_url = os.environ.get("ASSISTANT_API_URL", "http://api:8080")
        assistant_api_url = assistant_api_url.rstrip("/")

        timeout_seconds = int(os.environ.get("ASSISTANT_INGEST_TIMEOUT_SECONDS", "240"))
        poll_interval = float(os.environ.get("ASSISTANT_INGEST_POLL_SECONDS", "2"))
        default_question = os.environ.get(
            "ASSISTANT_DEFAULT_SUMMARY_QUESTION",
            "Сделай краткое summary документа и перечисли ключевые факты.",
        )

        rag_question = (question or "").strip() or default_question

        openwebui_base = self._detect_openwebui_base(__request__)
        auth_headers, cookies = self._auth_context(__request__)

        results = []
        for file_item in files:
            result = {
                "file_name": file_item.get("name") or "unknown",
                "file_id": self._extract_file_id(file_item),
                "upload_status": "not_started",
                "index_status": "not_started",
                "rag_status": "not_started",
            }

            file_id = result["file_id"]
            if not file_id:
                result["error"] = "Failed to resolve file id from attachment"
                results.append(result)
                continue

            try:
                binary_payload, content_type = self._download_file(
                    openwebui_base,
                    file_id,
                    auth_headers,
                    cookies,
                )

                upload_response = self._upload_to_assistant(
                    assistant_api_url,
                    result["file_name"],
                    binary_payload,
                    content_type,
                )
                result["upload_status"] = "uploaded"
                document_id = upload_response.get("id")
                result["document_id"] = document_id

                document = self._poll_document_status(
                    assistant_api_url,
                    document_id,
                    timeout_seconds,
                    poll_interval,
                )

                status = document.get("status")
                if status != "ready":
                    result["index_status"] = status or "unknown"
                    result["error"] = document.get("error") or "Document processing failed"
                    results.append(result)
                    continue

                result["index_status"] = "ready"
                rag_response = self._query_rag(assistant_api_url, rag_question)
                result["rag_status"] = "ok"
                result["answer"] = rag_response.get("text")
                result["sources"] = rag_response.get("sources", [])
                result["debug"] = {
                    "question": rag_question,
                    "source_count": len(rag_response.get("sources", [])),
                }
            except Exception as exc:
                result["error"] = str(exc)
            results.append(result)

        return json.dumps(
            {
                "question": rag_question,
                "results": results,
            },
            ensure_ascii=False,
        )

    def _detect_openwebui_base(self, request) -> str:
        if request is None:
            return os.environ.get("OPENWEBUI_BASE_URL", "http://openwebui:8080").rstrip("/")
        return str(request.base_url).rstrip("/")

    def _auth_context(self, request):
        headers = {}
        cookies = {}
        if request is None:
            return headers, cookies

        authorization = request.headers.get("authorization")
        if authorization:
            headers["Authorization"] = authorization

        token_cookie = request.cookies.get("token")
        if token_cookie and "Authorization" not in headers:
            cookies["token"] = token_cookie

        return headers, cookies

    def _extract_file_id(self, file_item: Dict[str, Any]) -> str:
        value = file_item.get("id") or file_item.get("url")
        if value is None:
            return ""
        value = str(value).strip()
        if "/" in value:
            value = value.rsplit("/", 1)[-1]
        return value

    def _download_file(self, base_url: str, file_id: str, headers: Dict[str, str], cookies: Dict[str, str]):
        response = requests.get(
            f"{base_url}/api/v1/files/{file_id}/content",
            headers=headers,
            cookies=cookies,
            timeout=60,
        )
        if response.status_code >= 300:
            raise RuntimeError(f"OpenWebUI file download failed: {response.status_code} {response.text}")
        content_type = response.headers.get("Content-Type", "application/octet-stream")
        return response.content, content_type

    def _upload_to_assistant(self, assistant_api_url: str, filename: str, payload: bytes, content_type: str):
        files = {
            "file": (filename, payload, content_type),
        }
        response = requests.post(f"{assistant_api_url}/v1/documents", files=files, timeout=120)
        if response.status_code >= 300:
            raise RuntimeError(f"Assistant upload failed: {response.status_code} {response.text}")
        return response.json()

    def _poll_document_status(self, assistant_api_url: str, document_id: str, timeout_seconds: int, poll_interval: float):
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
            time.sleep(poll_interval)
        raise RuntimeError(f"Timeout waiting for document {document_id} to reach ready status")

    def _query_rag(self, assistant_api_url: str, question: str):
        payload = {
            "question": question,
        }
        response = requests.post(f"{assistant_api_url}/v1/rag/query", json=payload, timeout=120)
        if response.status_code >= 300:
            raise RuntimeError(f"RAG query failed: {response.status_code} {response.text}")
        return response.json()
