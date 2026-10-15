# Copyright (C) 2023-2026 QuantumNous
# Copyright (C) 2026 CubeRouter
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as
# published by the Free Software Foundation, either version 3 of the
# License, or (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program. If not, see <https://www.gnu.org/licenses/>.
#
# For commercial licensing, please contact support@quantumnous.com
"""Authenticated image-only API. Images remain in bounded process memory."""
import asyncio
import hmac
import os
import threading
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import APIRouter, FastAPI, Request
from fastapi.responses import JSONResponse
from starlette.concurrency import run_in_threadpool

from moderation import InvalidImage, MAX_BYTES, ShieldRuntime


def router_for(runtime):
    router = APIRouter()
    capacity = threading.BoundedSemaphore(8)

    def response(data, status=200):
        return JSONResponse(data, status_code=status, headers={"Cache-Control": "no-store"})

    def error(status=503):
        return response({"decision": "error", "release_allowed": False}, status)

    def authenticated(request):
        # A dedicated secret, never the HF download token or a CubeRouter key.
        try:
            token = Path(os.environ["SHIELDGEMMA_TOKEN_FILE"]).read_text().strip()
        except (KeyError, OSError):
            return False
        supplied = request.headers.get("authorization", "")
        return (32 <= len(token) <= 256 and token.isascii() and
                hmac.compare_digest(supplied.encode(), ("Bearer " + token).encode()))

    @router.get("/v1/health")
    def health(request: Request):
        if not authenticated(request):
            return error(401)
        state = runtime.health()
        return response(state, 200 if state["ready"] else 503)

    @router.post("/v1/moderate")
    async def moderate(request: Request):
        if not authenticated(request):
            return error(401)
        if request.headers.get("content-type", "").split(";", 1)[0] != "image/png":
            return error(415)
        if runtime.status != "ready" or not runtime.weights_verified:
            return error()
        if not capacity.acquire(blocking=False):
            return error(503)

        async def read_image():
            data = bytearray()
            async for chunk in request.stream():
                if len(data) + len(chunk) > MAX_BYTES:
                    raise InvalidImage("size")
                data.extend(chunk)
            return bytes(data)

        def check(data):
            # Hold the gate inside the worker thread, including after a caller
            # disconnects. Never start concurrent inference on this GPU.
            from moderation import decode_image
            if not runtime.gate.acquire(timeout=30):
                raise RuntimeError("busy")
            try:
                image = decode_image(data)
                try:
                    result = runtime.classify(image, data)
                    return {**result, "protocol": "image-moderation-v1",
                            "weights_verified": runtime.weights_verified}
                finally:
                    image.close()
            finally:
                runtime.gate.release()

        try:
            data = await asyncio.wait_for(read_image(), timeout=20)
            return response(await run_in_threadpool(check, data))
        except InvalidImage:
            return error(400)
        except Exception:
            # Do not expose image data, tokens, tracebacks or model errors.
            return error()
        finally:
            capacity.release()

    return router


runtime = ShieldRuntime(Path(os.environ.get("SHIELDGEMMA_MODEL_DIR", "models/shieldgemma-2-4b-it")))


@asynccontextmanager
async def lifespan(app):
    threading.Thread(target=runtime.load, daemon=True).start()
    yield


app = FastAPI(lifespan=lifespan, docs_url=None, redoc_url=None, openapi_url=None)
app.include_router(router_for(runtime))
