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

"""Local OpenAI Images adapter for the Qwen Image Studio workflow API.

Only the configured upstream is contacted. The browser receives PNGs inline;
upstream asset paths are validated and never used as arbitrary fetch URLs.
"""

import base64
import hmac
import json
import logging
import math
import os
import re
import struct
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, ProxyHandler, Request, build_opener

MODEL = "qwen-image-2512"
SIZES = {
    "1328x1328": "1:1", "1664x928": "16:9", "928x1664": "9:16",
    "1472x1104": "4:3", "1104x1472": "3:4", "1584x1056": "3:2",
    "1056x1584": "2:3",
}
QUALITY_STEPS = {"fast": 20, "standard": 30, "high": 50}
MAX_BODY = 65536
MAX_IMAGE = 20 * 1024 * 1024
PNG_MAGIC = b"\x89PNG\r\n\x1a\n"


class BridgeError(Exception):
    def __init__(self, status, message, code="upstream_error"):
        super().__init__(message)
        self.status = status
        self.code = code


def to_workflow(body):
    if not isinstance(body, dict):
        raise BridgeError(400, "Expected a JSON object", "invalid_request")
    allowed = {"model", "prompt", "n", "size", "quality", "seed",
               "num_inference_steps", "true_cfg_scale", "response_format", "user"}
    if set(body) - allowed:
        raise BridgeError(400, "Unsupported fields: " + ", ".join(sorted(set(body) - allowed)), "invalid_request")
    if body.get("model") != MODEL:
        raise BridgeError(400, "This adapter serves " + MODEL, "invalid_model")
    prompt = body.get("prompt")
    if not isinstance(prompt, str) or not prompt.strip() or len(prompt) > 16000:
        raise BridgeError(400, "prompt must contain 1-16000 characters", "invalid_request")
    count = body.get("n", 1)
    if type(count) is not int or not 1 <= count <= 4:
        raise BridgeError(400, "n must be an integer from 1 to 4", "invalid_request")
    size = body.get("size", "1664x928")
    if not isinstance(size, str) or size not in SIZES:
        raise BridgeError(400, "Unsupported size. Use one of: " + ", ".join(SIZES), "invalid_request")
    quality = body.get("quality", "standard")
    if not isinstance(quality, str) or quality not in QUALITY_STEPS:
        raise BridgeError(400, "quality must be fast, standard or high", "invalid_request")
    steps = body.get("num_inference_steps", QUALITY_STEPS[quality])
    seed = body.get("seed", 42)
    cfg = body.get("true_cfg_scale", 1)
    if type(steps) is not int or not 1 <= steps <= 100:
        raise BridgeError(400, "num_inference_steps must be 1-100", "invalid_request")
    if type(seed) is not int or not 0 <= seed <= 2**63 - count:
        raise BridgeError(400, "Invalid seed for the requested batch", "invalid_request")
    if type(cfg) not in (int, float) or not math.isfinite(cfg) or not 0 <= cfg <= 10:
        raise BridgeError(400, "true_cfg_scale must be between 0 and 10", "invalid_request")
    if body.get("response_format", "b64_json") != "b64_json":
        raise BridgeError(400, "Use response_format=b64_json; this adapter returns inline PNGs", "invalid_request")
    return {"mode": "create", "parameters": {
        "prompt": prompt.strip(), "aspect_ratio": SIZES[size],
        "num_images_per_prompt": count, "num_inference_steps": steps,
        "seed": seed, "true_cfg_scale": cfg,
    }}


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise BridgeError(502, "Upstream redirect rejected", "invalid_upstream_response")


class WorkflowClient:
    def __init__(self, origin, deadline=540, poll_seconds=1):
        parsed = urlsplit(origin)
        if parsed.scheme not in ("http", "https") or not parsed.hostname or parsed.username or parsed.password:
            raise ValueError("QWEN_WORKFLOW_ORIGIN must be an HTTP(S) origin without credentials")
        if parsed.path not in ("", "/") or parsed.query or parsed.fragment:
            raise ValueError("QWEN_WORKFLOW_ORIGIN must not contain a path, query or fragment")
        self.origin = origin.rstrip("/")
        self.deadline = deadline
        self.poll_seconds = poll_seconds
        self.http = build_opener(ProxyHandler({}), NoRedirect())

    def request(self, path, body=None, image=False, timeout=30, content_type="application/json"):
        allowed = (r"/api/workflow/(generate|jobs/[0-9a-f]{32}|comparison/[0-9a-f]{32}|comparison-media/[0-9a-f]{32}/[0-9a-f]{32}\.png)"
                   r"|/media/[0-9a-f]{32}\.png|/api/uploads|/api/(nodes/162|edit|tools)/health"
                   r"|/api/tools/(ocr|preview|text|regional|masks|jobs/[0-9a-f]{32})")
        if not re.fullmatch(allowed, path):
            raise BridgeError(502, "Unexpected workflow path", "invalid_upstream_response")
        data = body if isinstance(body, bytes) or body is None else json.dumps(body, ensure_ascii=False, allow_nan=False).encode()
        req = Request(self.origin + path, data=data, headers={"Content-Type": content_type})
        try:
            with self.http.open(req, timeout=timeout) as response:
                limit = MAX_IMAGE if image else 2 * 1024 * 1024
                raw = response.read(limit + 1)
                if len(raw) > limit:
                    raise BridgeError(502, "Upstream response exceeds size limit", "invalid_upstream_response")
                return raw if image else json.loads(raw)
        except HTTPError as error:
            status = 429 if error.code in (409, 429) else 502
            if error.code in (400, 422):
                status = 400
            raise BridgeError(status, "Workflow rejected the request (HTTP %s)" % error.code) from error
        except (URLError, TimeoutError, OSError) as error:
            raise BridgeError(502, "Workflow connection failed; check the SSH tunnel. A submitted job may still be running.") from error
        except (ValueError, UnicodeDecodeError) as error:
            raise BridgeError(502, "Invalid workflow JSON", "invalid_upstream_response") from error

    def generate(self, body):
        workflow = to_workflow(body)
        started = time.monotonic()
        accepted = self.request("/api/workflow/generate", workflow)
        job_id = accepted.get("job_id") if isinstance(accepted, dict) else None
        if not isinstance(job_id, str) or not re.fullmatch(r"[0-9a-f]{32}", job_id):
            raise BridgeError(502, "Workflow did not return a valid job ID", "invalid_upstream_response")
        logging.info("accepted job=%s count=%s ratio=%s", job_id,
                     workflow["parameters"]["num_images_per_prompt"], workflow["parameters"]["aspect_ratio"])
        while True:
            remaining = self.deadline - (time.monotonic() - started)
            if remaining <= 0:
                raise BridgeError(409, "Workflow %s is still running; timeout does not cancel generation." % job_id, "workflow_pending")
            job = self.request("/api/workflow/jobs/" + job_id, timeout=min(30, remaining))
            if not isinstance(job, dict) or job.get("job_id") != job_id:
                raise BridgeError(502, "Unexpected workflow job response", "invalid_upstream_response")
            state = job.get("state")
            if state == "completed":
                result = self.to_openai(job, body)
                logging.info("completed job=%s images=%s seconds=%.3f", job_id, len(result["data"]), time.monotonic() - started)
                return result
            if state in ("failed", "needs_review"):
                raise BridgeError(422, "Workflow %s ended with state %s. Check the Image Studio job." % (job_id, state), "workflow_" + state)
            if state not in ("queued", "running"):
                raise BridgeError(502, "Unknown workflow state", "invalid_upstream_response")
            time.sleep(min(self.poll_seconds, max(0, remaining)))

    def to_openai(self, job, body):
        result = job.get("result")
        images = result.get("images") if isinstance(result, dict) else None
        if not isinstance(images, list) or len(images) != body.get("n", 1):
            raise BridgeError(502, "Workflow returned an unexpected image count", "invalid_upstream_response")
        width, height = map(int, body.get("size", "1664x928").split("x"))
        output = []
        for item in images:
            path = item.get("image_url") if isinstance(item, dict) else None
            if not isinstance(path, str) or not re.fullmatch(r"/media/[0-9a-f]{32}\.png", path):
                raise BridgeError(502, "Unexpected image path", "invalid_upstream_response")
            raw = self.request(path, image=True)
            if len(raw) < 24 or not raw.startswith(PNG_MAGIC) or raw[12:16] != b"IHDR":
                raise BridgeError(502, "Workflow asset is not PNG", "invalid_upstream_response")
            if struct.unpack(">II", raw[16:24]) != (width, height):
                raise BridgeError(502, "Image dimensions differ from the request", "invalid_upstream_response")
            output.append({"b64_json": base64.b64encode(raw).decode("ascii")})
        return {"created": int(time.time()), "data": output, "workflow": {
            "job_id": job["job_id"], "model": MODEL, "images": images,
            "inference_seconds": result.get("inference_seconds"),
            "workflow_seconds": result.get("workflow_seconds"),
            "text_quality": result.get("text_quality"),
            "quality": job.get("quality", []), "notice": result.get("notice"),
        }}


class BridgeServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, address, client, key, studio=None):
        super().__init__(address, Handler)
        self.client = client
        self.key = key
        self.generation_lock = threading.Lock()
        self.studio = studio


class Handler(BaseHTTPRequestHandler):
    def reply(self, status, body, content_type="application/json; charset=utf-8"):
        raw = body if isinstance(body, bytes) else json.dumps(body, ensure_ascii=False, allow_nan=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(raw)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        try:
            self.wfile.write(raw)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def error(self, error):
        kind = "invalid_request_error" if error.status in (400, 401, 404, 422) else "server_error"
        self.reply(error.status, {"error": {"message": str(error), "type": kind, "code": error.code}})

    def authenticated(self):
        supplied = self.headers.get("Authorization", "").encode()
        expected = ("Bearer " + self.server.key).encode()
        if not hmac.compare_digest(supplied, expected):
            self.error(BridgeError(401, "Invalid adapter key", "invalid_api_key"))
            return False
        return True

    def do_GET(self):
        if self.path.startswith("/studio/"):
            self.studio_request()
        elif self.path == "/health":
            self.reply(200, {"state": "ready", "model": MODEL, "busy": self.server.generation_lock.locked(), "upstream_checked": False})
        elif self.path == "/v1/models" and self.authenticated():
            models = [MODEL, "qwen-image-edit-2511"] if self.server.studio else [MODEL]
            self.reply(200, {"object": "list", "data": [{"id": model, "object": "model", "owned_by": "local-qwen-workflow"} for model in models]})
        elif self.path != "/v1/models":
            self.error(BridgeError(404, "Unknown endpoint", "not_found"))

    def do_POST(self):
        if self.path.startswith("/studio/"):
            self.studio_request()
            return
        if not self.authenticated():
            return
        if self.path != "/v1/images/generations":
            self.error(BridgeError(404, "Only text-to-image generation is currently supported", "not_found"))
            return
        acquired = False
        try:
            if self.headers.get("Transfer-Encoding"):
                raise BridgeError(400, "Chunked requests are unsupported", "invalid_request")
            if self.headers.get_content_type() != "application/json":
                raise BridgeError(400, "Expected application/json", "invalid_request")
            size = int(self.headers.get("Content-Length", "0"))
            if not 0 < size <= MAX_BODY:
                raise BridgeError(400, "Request must contain 1-65536 bytes", "invalid_request")
            self.connection.settimeout(30)
            raw = self.rfile.read(size)
            if len(raw) != size:
                raise BridgeError(400, "Incomplete request", "invalid_request")
            body = json.loads(raw)
            if self.server.studio and isinstance(body, dict) and isinstance(body.get("extra_fields"), dict):
                self.reply(200, self.server.studio.run_relay(body))
                return
            to_workflow(body)
            acquired = self.server.generation_lock.acquire(False)
            if not acquired:
                raise BridgeError(429, "An image generation is already running", "busy")
            self.reply(200, self.server.client.generate(body))
        except BridgeError as error:
            self.error(error)
        except (ValueError, UnicodeDecodeError, TimeoutError) as error:
            self.error(BridgeError(400, "Invalid JSON request", "invalid_request"))
        except Exception:
            logging.exception("adapter request failed")
            self.error(BridgeError(502, "Image adapter could not complete the request"))
        finally:
            if acquired:
                self.server.generation_lock.release()

    def log_message(self, format_string, *args):
        logging.info(format_string, *args)

    def do_DELETE(self):
        self.studio_request()

    def studio_request(self):
        if not self.authenticated():
            return
        if not self.server.studio:
            self.error(BridgeError(503, "Studio storage is not configured", "unavailable"))
            return
        try:
            body = None
            kind = self.headers.get_content_type()
            if self.command == "POST":
                size = int(self.headers.get("Content-Length", "0"))
                if self.headers.get("Transfer-Encoding") or not 0 < size <= 15 * 1024**2:
                    raise BridgeError(413, "Invalid studio request size", "invalid_request")
                self.connection.settimeout(30)
                body = self.rfile.read(size)
                if len(body) != size:
                    raise BridgeError(400, "Incomplete request", "invalid_request")
                if kind == "application/json":
                    body = json.loads(body)
            result, output_kind = self.server.studio.handle(self.command, self.path, body, kind)
            self.reply(200, result, output_kind)
        except BridgeError as error:
            self.error(error)
        except (ValueError, TypeError, KeyError, UnicodeDecodeError):
            self.error(BridgeError(400, "Invalid studio request", "invalid_request"))
        except Exception:
            logging.exception("studio operation failed")
            self.error(BridgeError(502, "Studio could not complete the operation", "studio_error"))


if __name__ == "__main__":
    sys.modules["bridge"] = sys.modules[__name__]
    from studio import Studio
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(message)s")
    key = os.environ.get("QWEN_BRIDGE_API_KEY", "")
    if len(key) < 24:
        raise SystemExit("Set QWEN_BRIDGE_API_KEY to a random local key of at least 24 characters")
    client = WorkflowClient(os.environ.get("QWEN_WORKFLOW_ORIGIN", "http://127.0.0.1:18162"))
    port = int(os.environ.get("QWEN_BRIDGE_PORT", "18163"))
    logging.info("Qwen workflow adapter listening on 127.0.0.1:%s", port)
    storage = os.environ.get("QWEN_STUDIO_DATA")
    studio = Studio(storage, client, int(os.environ.get("QWEN_STUDIO_RETENTION_DAYS", "30"))) if storage else None
    BridgeServer(("127.0.0.1", port), client, key, studio).serve_forever()
