"""
title: Personal AI Assistant RAG Pipe
author: personal-ai-assistant
author_url: https://github.com/kirillkom/personal-ai-assistant
description: Pipe function that intercepts file uploads, sends them through PAA ingest pipeline, and proxies chat to PAA /v1/chat/completions with built-in hybrid RAG.
required_open_webui_version: 0.6.0
version: 1.0.0
"""

import asyncio
import json
import time
from typing import Any, AsyncGenerator, Dict, List, Optional, Union

import aiohttp
from pydantic import BaseModel, Field


class Pipe:
    class Valves(BaseModel):
        ASSISTANT_API_URL: str = Field(
            default="http://api:8080",
            description="URL of the PAA backend API.",
        )
        ASSISTANT_API_KEY: str = Field(
            default="",
            description="Bearer token for PAA API (OPENAI_COMPAT_API_KEY). Leave empty if auth is disabled.",
        )
        ASSISTANT_MODEL_ID: str = Field(
            default="paa-rag-v1",
            description="Model ID to send to PAA /v1/chat/completions.",
        )
        INGEST_TIMEOUT_SECONDS: int = Field(
            default=120,
            description="Max seconds to wait for document processing.",
        )
        INGEST_POLL_SECONDS: float = Field(
            default=2.0,
            description="Polling interval for document status.",
        )
        STATUS_UPDATES: bool = Field(
            default=True,
            description="Show progress updates in chat while processing files.",
        )

    def __init__(self):
        self.valves = self.Valves()
        self.file_handler = True

    def pipes(self) -> list[dict]:
        try:
            import requests

            url = f"{self.valves.ASSISTANT_API_URL.rstrip('/')}/v1/models"
            headers = {}
            if self.valves.ASSISTANT_API_KEY:
                headers["Authorization"] = f"Bearer {self.valves.ASSISTANT_API_KEY}"
            resp = requests.get(url, headers=headers, timeout=5)
            if resp.ok:
                data = resp.json().get("data", [])
                if data:
                    return [{"id": m["id"], "name": m.get("id", "PAA")} for m in data]
        except Exception:
            pass
        # Fallback — single model as before.
        return [
            {
                "id": self.valves.ASSISTANT_MODEL_ID,
                "name": "Personal AI Assistant",
            }
        ]

    async def inlet(
        self,
        body: dict,
        __user__: Optional[dict] = None,
        __event_emitter__=None,
        __request__=None,
    ) -> dict:
        files = body.get("files", [])
        if not files:
            return body

        api_url = self.valves.ASSISTANT_API_URL.rstrip("/")
        openwebui_base = self._detect_openwebui_base(__request__)
        auth_headers, cookies = self._auth_context(__request__)

        async with aiohttp.ClientSession() as session:
            for file_item in files:
                file_name = file_item.get("name") or file_item.get("filename") or "unknown"
                file_id = self._extract_file_id(file_item)

                if not file_id:
                    continue

                if __event_emitter__ and self.valves.STATUS_UPDATES:
                    await __event_emitter__(
                        {
                            "type": "status",
                            "data": {
                                "description": f"Uploading: {file_name}...",
                                "done": False,
                            },
                        }
                    )

                try:
                    file_bytes, content_type = await self._download_file(
                        session, openwebui_base, file_id, auth_headers, cookies
                    )

                    document_id = await self._upload_to_assistant(
                        session, api_url, file_name, file_bytes, content_type
                    )

                    if __event_emitter__ and self.valves.STATUS_UPDATES:
                        await __event_emitter__(
                            {
                                "type": "status",
                                "data": {
                                    "description": f"Indexing: {file_name}...",
                                    "done": False,
                                },
                            }
                        )

                    await self._poll_document_ready(session, api_url, document_id)

                    if __event_emitter__ and self.valves.STATUS_UPDATES:
                        await __event_emitter__(
                            {
                                "type": "status",
                                "data": {
                                    "description": f"Ready: {file_name}",
                                    "done": False,
                                },
                            }
                        )

                except Exception as exc:
                    if __event_emitter__:
                        await __event_emitter__(
                            {
                                "type": "status",
                                "data": {
                                    "description": f"Error ({file_name}): {exc}",
                                    "done": True,
                                },
                            }
                        )

        body.pop("files", None)

        if __event_emitter__ and self.valves.STATUS_UPDATES:
            await __event_emitter__(
                {
                    "type": "status",
                    "data": {
                        "description": "Files processed. Running RAG query...",
                        "done": True,
                    },
                }
            )

        return body

    async def pipe(
        self,
        body: dict,
        __user__: Optional[dict] = None,
        __event_emitter__=None,
    ) -> Union[str, AsyncGenerator[str, None]]:
        api_url = self.valves.ASSISTANT_API_URL.rstrip("/")
        headers = {"Content-Type": "application/json"}
        if self.valves.ASSISTANT_API_KEY:
            headers["Authorization"] = f"Bearer {self.valves.ASSISTANT_API_KEY}"

        user_id = "default"
        if __user__:
            user_id = __user__.get("id") or __user__.get("email") or "default"

        # Use the model selected in the UI dropdown (strip pipe prefix added by OpenWebUI).
        selected_model = body.get("model", self.valves.ASSISTANT_MODEL_ID)
        if selected_model and "." in selected_model:
            selected_model = selected_model.split(".", 1)[1]

        payload = {
            "model": selected_model or self.valves.ASSISTANT_MODEL_ID,
            "messages": body.get("messages", []),
            "stream": body.get("stream", True),
            "metadata": {
                "user_id": user_id,
                "conversation_id": body.get("chat_id", ""),
            },
        }

        if body.get("stream", True):
            return self._stream_response(api_url, headers, payload)
        else:
            return await self._blocking_response(api_url, headers, payload)

    async def _stream_response(
        self, api_url: str, headers: dict, payload: dict
    ) -> AsyncGenerator[str, None]:
        try:
            async with aiohttp.ClientSession() as session:
                async with session.post(
                    f"{api_url}/v1/chat/completions",
                    json=payload,
                    headers=headers,
                    timeout=aiohttp.ClientTimeout(total=300),
                ) as resp:
                    if resp.status >= 300:
                        text = await resp.text()
                        yield f"Error from PAA API: {resp.status} {text}"
                        return

                    buffer = ""
                    async for chunk in resp.content.iter_any():
                        buffer += chunk.decode("utf-8", errors="replace")
                        while "\n" in buffer:
                            line, buffer = buffer.split("\n", 1)
                            line = line.strip()
                            if not line or not line.startswith("data:"):
                                continue
                            data = line[len("data:"):].strip()
                            if data == "[DONE]":
                                return
                            try:
                                obj = json.loads(data)
                                content = obj["choices"][0]["delta"].get("content", "")
                                if content:
                                    yield content
                            except (json.JSONDecodeError, KeyError, IndexError):
                                continue
        except Exception as exc:
            yield f"Error: {exc}"

    async def _blocking_response(
        self, api_url: str, headers: dict, payload: dict
    ) -> str:
        async with aiohttp.ClientSession() as session:
            async with session.post(
                f"{api_url}/v1/chat/completions",
                json=payload,
                headers=headers,
                timeout=aiohttp.ClientTimeout(total=300),
            ) as resp:
                if resp.status >= 300:
                    text = await resp.text()
                    return f"Error from PAA API: {resp.status} {text}"

                data = await resp.json()
                choices = data.get("choices", [])
                if choices:
                    return choices[0].get("message", {}).get("content", "")
                return json.dumps(data, ensure_ascii=False)

    # --- Helper methods ---

    def _detect_openwebui_base(self, request) -> str:
        if request is None:
            return "http://openwebui:8080"
        return str(request.base_url).rstrip("/")

    def _auth_context(self, request):
        headers: Dict[str, str] = {}
        cookies: Dict[str, str] = {}
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

    def _assistant_headers(self) -> Dict[str, str]:
        headers: Dict[str, str] = {}
        if self.valves.ASSISTANT_API_KEY:
            headers["Authorization"] = f"Bearer {self.valves.ASSISTANT_API_KEY}"
        return headers

    async def _download_file(
        self,
        session: aiohttp.ClientSession,
        base_url: str,
        file_id: str,
        headers: Dict[str, str],
        cookies: Dict[str, str],
    ) -> tuple[bytes, str]:
        cookie_str = "; ".join(f"{k}={v}" for k, v in cookies.items()) if cookies else None
        req_headers = dict(headers)
        if cookie_str:
            req_headers["Cookie"] = cookie_str

        async with session.get(
            f"{base_url}/api/v1/files/{file_id}/content",
            headers=req_headers,
            timeout=aiohttp.ClientTimeout(total=60),
        ) as resp:
            if resp.status >= 300:
                text = await resp.text()
                raise RuntimeError(f"OpenWebUI file download failed: {resp.status} {text}")
            content_type = resp.headers.get("Content-Type", "application/octet-stream")
            data = await resp.read()
            return data, content_type

    async def _upload_to_assistant(
        self,
        session: aiohttp.ClientSession,
        api_url: str,
        filename: str,
        payload: bytes,
        content_type: str,
    ) -> str:
        form = aiohttp.FormData()
        form.add_field("file", payload, filename=filename, content_type=content_type)

        async with session.post(
            f"{api_url}/v1/documents",
            data=form,
            headers=self._assistant_headers(),
            timeout=aiohttp.ClientTimeout(total=120),
        ) as resp:
            if resp.status >= 300:
                text = await resp.text()
                raise RuntimeError(f"PAA upload failed: {resp.status} {text}")
            data = await resp.json()
            doc_id = data.get("id")
            if not doc_id:
                raise RuntimeError(f"PAA upload returned no document id: {data}")
            return doc_id

    async def _poll_document_ready(
        self, session: aiohttp.ClientSession, api_url: str, document_id: str
    ) -> None:
        deadline = time.time() + self.valves.INGEST_TIMEOUT_SECONDS
        while time.time() < deadline:
            async with session.get(
                f"{api_url}/v1/documents/{document_id}",
                headers=self._assistant_headers(),
                timeout=aiohttp.ClientTimeout(total=30),
            ) as resp:
                if resp.status >= 300:
                    text = await resp.text()
                    raise RuntimeError(
                        f"Document status check failed: {resp.status} {text}"
                    )
                data = await resp.json()
                status = data.get("status")
                if status == "ready":
                    return
                if status == "failed":
                    raise RuntimeError(
                        data.get("error") or "Document processing failed"
                    )
            await asyncio.sleep(self.valves.INGEST_POLL_SECONDS)

        raise RuntimeError(
            f"Timeout ({self.valves.INGEST_TIMEOUT_SECONDS}s) waiting for document {document_id}"
        )
