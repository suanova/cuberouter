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

"""Account-scoped media workflow. Private API is called only by CubeRouter.

GPU work requires a single-use preparation and CubeRouter's relay authorization.
Files become readable only after CubeRouter confirms relay completion. The
standard relay continues to own quota reservation, settlement and consume logs.
"""
import copy
import json
import os
import re
import secrets
import struct
import threading
import time
import uuid
from pathlib import Path

from bridge import BridgeError, MAX_IMAGE, PNG_MAGIC, SIZES

ID = re.compile(r"[0-9a-f]{32}")
MODELS = {"create": "qwen-image-2512", "edit": "qwen-image-edit-2511", "regional": "qwen-image-edit-2511"}
TERMINAL = {"completed", "failed", "needs_review", "interrupted"}


def identifier(value):
    if not isinstance(value, str) or not ID.fullmatch(value):
        raise BridgeError(404, "Unknown studio resource", "not_found")
    return value


def bounded_integer(value, low, high, name):
    if type(value) is not int or not low <= value <= high:
        raise BridgeError(400, f"{name} must be an integer from {low} to {high}", "invalid_request")
    return value


class Studio:
    def __init__(self, root, client, retention_days=30):
        self.root = Path(root).resolve()
        self.root.mkdir(parents=True, exist_ok=True)
        self.client = client
        self.retention_days = bounded_integer(retention_days, 1, 365, "retention_days")
        self.lock = threading.RLock()
        self.gpu_lock = threading.Lock()
        self.cpu_slots = threading.BoundedSemaphore(2)
        self.last_cleanup = 0
        # Never resubmit a possibly executed GPU operation on restart.
        for file in self.root.glob("*/jobs/*.json"):
            job = json.loads(file.read_text(encoding="utf-8"))
            if job["state"] not in TERMINAL:
                job.update(state="interrupted", stage="Worker restarted; check usage logs before retrying", error="interrupted")
                self.write(file, job)

    def owner_dir(self, owner):
        if not str(owner).isdigit() or not 0 < int(owner) < 2**53:
            raise BridgeError(401, "Invalid account", "invalid_account")
        root = self.root / str(int(owner))
        for kind in ("jobs", "assets", "masks"):
            (root / kind).mkdir(parents=True, exist_ok=True)
        return root

    def write(self, path, data):
        temporary = path.with_suffix("." + uuid.uuid4().hex + ".tmp")
        with temporary.open("x", encoding="utf-8") as handle:
            json.dump(data, handle, ensure_ascii=False, allow_nan=False)
        if os.name != "nt":
            temporary.chmod(0o600)
        temporary.replace(path)

    def load(self, owner, kind, ident):
        path = self.owner_dir(owner) / kind / (identifier(ident) + ".json")
        if not path.is_file():
            raise BridgeError(404, "Unknown studio resource", "not_found")
        item = json.loads(path.read_text(encoding="utf-8"))
        if item["expires_at"] < time.time():
            raise BridgeError(410, "Studio resource has expired", "expired")
        return item

    def save_job(self, owner, job):
        self.write(self.owner_dir(owner) / "jobs" / (job["id"] + ".json"), job)

    def asset(self, owner, ident, allow_pending=False):
        item = self.load(owner, "assets", ident)
        if item.get("job_id") and not allow_pending:
            parent = self.load(owner, "jobs", item["job_id"])
            if parent["state"] != "completed":
                raise BridgeError(409, "Result is not released yet", "result_pending")
        return item

    def public_asset(self, item):
        return {key: item[key] for key in ("id", "width", "height", "expires_at")}

    def persist_asset(self, owner, remote, job_id=None, dimensions=None):
        path = remote.get("image_url", "")
        raw = self.client.request(path, image=True)
        if len(raw) < 24 or raw[:8] != PNG_MAGIC or raw[12:16] != b"IHDR":
            raise BridgeError(502, "Worker returned an invalid PNG", "invalid_upstream_response")
        width, height = struct.unpack(">II", raw[16:24])
        if not 0 < width * height <= 16_000_000:
            raise BridgeError(502, "Worker image is too large", "invalid_upstream_response")
        if dimensions and (width, height) != dimensions:
            raise BridgeError(502, "Worker returned an unexpected image size", "invalid_upstream_response")
        ident = uuid.uuid4().hex
        item = {"id": ident, "remote_id": identifier(remote["asset_id"]), "width": width, "height": height,
                "job_id": job_id, "expires_at": time.time() + self.retention_days * 86400}
        directory = self.owner_dir(owner) / "assets"
        (directory / (ident + ".png")).write_bytes(raw)
        self.write(directory / (ident + ".json"), item)
        return self.public_asset(item)

    def quota(self, owner):
        directory = self.owner_dir(owner)
        total = sum(p.stat().st_size for p in (directory / "assets").glob("*.png"))
        # Bound storage and per-request additions before expensive remote work.
        if total > 2 * 1024**3 - 12 * MAX_IMAGE:
            raise BridgeError(429, "Media storage is full; delete old versions first", "storage_full")
        if len(list((directory / "jobs").glob("*.json"))) >= 1000:
            raise BridgeError(429, "Studio history is full; delete old versions first", "history_full")

    def cleanup(self):
        with self.lock:
            now = time.time()
            if now - self.last_cleanup < 3600:
                return
            self.last_cleanup = now
            for file in self.root.glob("*/*/*.json"):
                item = json.loads(file.read_text(encoding="utf-8"))
                if item.get("expires_at", now + 1) >= now:
                    continue
                if file.parent.name == "jobs" and item.get("state") not in TERMINAL | {"prepared"}:
                    continue
                file.with_suffix(".png").unlink(missing_ok=True)
                file.unlink(missing_ok=True)

    def public_job(self, job):
        keys = ("id", "mode", "state", "stage", "created_at", "expires_at", "request", "parent_id",
                "completed_steps", "total_steps", "error", "request_id", "billing")
        result = {key: copy.deepcopy(job[key]) for key in keys if key in job}
        if job["state"] == "completed":
            result["result"] = copy.deepcopy(job.get("result", {}))
        return result

    def prepare(self, owner, request):
        if not isinstance(request, dict) or set(request) - {"mode", "prompt", "size", "count", "steps", "seed", "cfg", "references", "parent_id", "negative_prompt", "expected_text", "mask_id", "edit_mode"}:
            raise BridgeError(400, "Unknown studio request field", "invalid_request")
        mode = request.get("mode")
        if mode not in MODELS:
            raise BridgeError(400, "Unknown image operation", "invalid_request")
        prompt = request.get("prompt", "")
        if not isinstance(prompt, str) or not 0 < len(prompt.strip()) <= 16000:
            raise BridgeError(400, "Provide a prompt of 1-16000 characters", "invalid_request")
        count = bounded_integer(request.get("count", 1), 1, 4, "count")
        steps = bounded_integer(request.get("steps", 40), 1, 100 if mode == "create" else 60, "steps")
        seed = bounded_integer(request.get("seed", 42), 0, 9007199254740987, "seed")
        cfg = request.get("cfg", 1)
        if type(cfg) not in (int, float) or not 0 <= cfg <= 10:
            raise BridgeError(400, "CFG must be between 0 and 10", "invalid_request")
        size = request.get("size", "1664x928" if mode == "create" else "auto")
        sizes = SIZES if mode == "create" else {"auto", "1024x1024", "1344x768", "768x1344"}
        if not isinstance(size, str) or size not in sizes:
            raise BridgeError(400, "Invalid image size", "invalid_request")
        references = request.get("references", [])
        if not isinstance(references, list) or len(references) > 3 or (mode != "create" and not references) or (mode == "create" and references):
            raise BridgeError(400, "Editing requires 1-3 reference images", "invalid_request")
        remote_refs = [self.asset(owner, ident)["remote_id"] for ident in references]
        parent = request.get("parent_id")
        if parent:
            old = self.load(owner, "jobs", parent)
            if old["state"] != "completed" or not any(a["id"] in references for a in old.get("result", {}).get("images", [])):
                raise BridgeError(400, "Previous version must supply a reference image", "invalid_request")
        negative = request.get("negative_prompt", " ")
        expected = request.get("expected_text", "")
        if not isinstance(negative, str) or len(negative) > 8000 or not isinstance(expected, str) or len(expected) > 10000:
            raise BridgeError(400, "Text exceeds the allowed length", "invalid_request")
        parameters = {"prompt": prompt.strip(), "num_images_per_prompt": count, "num_inference_steps": steps, "seed": seed, "true_cfg_scale": cfg}
        if mode == "create":
            parameters["aspect_ratio"] = SIZES[size]
        else:
            parameters.update(image_asset_ids=remote_refs, negative_prompt=negative)
            if size != "auto":
                parameters["width"], parameters["height"] = map(int, size.split("x"))
        remote_body = {"mode": mode, "parameters": parameters, "expected_text": expected}
        if mode == "regional":
            if count != 1 or len(references) != 1 or cfg not in (1, 4) or steps < 8 or len(prompt) > 8000:
                raise BridgeError(400, "Regional edit requires one image, 8-60 steps and CFG 1 or 4", "invalid_request")
            mask = self.load(owner, "masks", request.get("mask_id"))
            edit_mode = request.get("edit_mode", "auto")
            if mask["asset_id"] != references[0] or edit_mode not in {"auto", "add", "replace", "remove", "appearance"}:
                raise BridgeError(400, "Invalid regional selection", "invalid_request")
            remote_body = {"asset_id": remote_refs[0], "mask_id": mask["remote_id"], "prompt": prompt.strip(), "num_inference_steps": steps,
                           "true_cfg_scale": cfg, "seed": seed, "edit_mode": edit_mode}
        with self.lock:
            self.quota(owner)
            for file in (self.owner_dir(owner) / "jobs").glob("*.json"):
                active = json.loads(file.read_text(encoding="utf-8"))
                if active["state"] == "prepared":
                    active.update(state="failed", stage="Replaced before submission", billing="relay_failed")
                    active.pop("relay_body", None)
                    self.save_job(owner, active)
                    continue
                if active["state"] not in TERMINAL and time.time() - active["created_at"] < 1200:
                    raise BridgeError(409, "Finish the current image job before submitting another", "busy")
            ident, ticket = uuid.uuid4().hex, secrets.token_urlsafe(32)
            # Steps determine the priced tier; clients cannot request high steps at a fast-tier price.
            quality = "high" if steps > 40 else "standard"
            if steps <= 20:
                quality = "fast"
            body = {"model": MODELS[mode], "prompt": prompt.strip(), "n": count, "size": size, "quality": quality,
                    "num_inference_steps": steps, "true_cfg_scale": cfg, "seed": seed, "response_format": "url",
                    "extra_fields": {"studio_job_id": ident, "studio_owner": int(owner), "studio_token": ticket}}
            job = {"id": ident, "owner": int(owner), "mode": mode, "state": "prepared", "stage": "Ready to submit",
                   "request": request, "parent_id": parent, "remote_body": remote_body, "relay_body": body,
                   "created_at": time.time(), "expires_at": time.time() + self.retention_days * 86400,
                   "total_steps": steps, "billing": "pending"}
            self.save_job(owner, job)
            return {"job": self.public_job(job), "relay_body": body}

    def authorize(self, owner, ident, body):
        with self.lock:
            job = self.load(owner, "jobs", ident)
            if job["state"] != "prepared" or time.time() - job["created_at"] > 600 or body != job["relay_body"]:
                raise BridgeError(409, "Job is stale or does not match the prepared request", "invalid_ticket")
            job.update(state="authorized", stage="Checking quota and submitting")
            self.save_job(owner, job)
        return {"ok": True}

    def settle(self, owner, ident, body):
        with self.lock:
            job = self.load(owner, "jobs", ident)
            success = body.get("success") is True
            if job["state"] == "completed":
                return {"ok": True}
            if success and job["state"] != "awaiting_settlement":
                raise BridgeError(409, "Generation has not completed", "not_ready")
            job.update(state="completed" if success else "failed", billing="relay_completed" if success else "relay_failed",
                       stage="Completed" if success else "Relay failed; no result released", request_id=body.get("request_id", ""))
            job.pop("relay_body", None)
            self.save_job(owner, job)
        return {"ok": True}

    def update(self, owner, ident, **changes):
        with self.lock:
            job = self.load(owner, "jobs", ident)
            job.update(changes)
            self.save_job(owner, job)

    def run_relay(self, body):
        extra = body.get("extra_fields", {})
        owner, ident = extra.get("studio_owner"), identifier(extra.get("studio_job_id"))
        with self.lock:
            job = self.load(owner, "jobs", ident)
            if job["state"] != "authorized" or body != job.get("relay_body"):
                raise BridgeError(409, "Studio request was not authorized or has already run", "invalid_ticket")
            if not self.gpu_lock.acquire(False):
                raise BridgeError(429, "Image worker is busy", "busy")
            job.update(state="running", stage="Submitting to image model")
            self.save_job(owner, job)
        try:
            self.execute(owner, job)
            final = self.load(owner, "jobs", ident)
            return {"created": int(time.time()), "data": [{"url": "/api/v1/media-studio/assets/" + image["id"] + "/content"} for image in final["result"]["images"]],
                    "studio": {"job_id": ident, "mode": job["mode"]}}
        except Exception:
            self.update(owner, ident, state="failed", stage="Generation failed", error="Image worker could not complete this job. No automatic retry was submitted.")
            raise
        finally:
            self.gpu_lock.release()

    def execute(self, owner, job):
        is_tool = job["mode"] in {"regional", "text"}
        submit = "/api/tools/" + job["mode"] if is_tool else "/api/workflow/generate"
        accepted = self.client.request(submit, job["remote_body"])
        remote_id = identifier(accepted.get("job_id"))
        self.update(owner, job["id"], remote_job_id=remote_id)
        status_path = ("/api/tools/jobs/" if is_tool else "/api/workflow/jobs/") + remote_id
        deadline = time.monotonic() + self.client.deadline
        while time.monotonic() < deadline:
            remote = self.client.request(status_path)
            if remote.get("job_id") != remote_id:
                raise BridgeError(502, "Unexpected worker job", "invalid_upstream_response")
            self.update(owner, job["id"], stage=remote.get("stage", "Generating"), completed_steps=remote.get("completed_steps", 0))
            if remote["state"] == "completed":
                result = self.collect(owner, job, remote)
                with self.lock:
                    latest = self.load(owner, "jobs", job["id"])
                    if latest.get("billing") == "relay_failed":
                        raise BridgeError(409, "Relay failed before generation completed; result remains private", "settlement_failed")
                    self.update(owner, job["id"], result=result, state="completed" if job["mode"] == "text" else "awaiting_settlement",
                                stage="Completed" if job["mode"] == "text" else "Finalizing usage log", billing="cpu_tool" if job["mode"] == "text" else "pending")
                return
            if remote["state"] in {"failed", "needs_review"}:
                raise BridgeError(422, "Image operation needs review; inspect the worker log using the studio job ID", "workflow_" + remote["state"])
            time.sleep(self.client.poll_seconds)
        raise BridgeError(409, "Worker timed out. Generation may still be running; do not automatically resubmit.", "workflow_pending")

    def collect(self, owner, job, remote):
        output = remote["result"]
        expected = job["request"].get("count", 1)
        if len(output.get("images", [])) != expected:
            raise BridgeError(502, "Unexpected output count", "invalid_upstream_response")
        dimensions = None
        if job["mode"] in {"text", "regional"}:
            source = self.asset(owner, job["request"]["references"][0])
            dimensions = (source["width"], source["height"])
        elif job["request"].get("size", "auto") != "auto":
            dimensions = tuple(map(int, job["request"]["size"].split("x")))
        images = [self.persist_asset(owner, a, job["id"], dimensions) for a in output["images"]]
        comparison = []
        if job["mode"] in {"create", "edit"}:
            records = self.client.request("/api/workflow/comparison/" + remote["job_id"])
            for row in records.get("images", []):
                if not row.get("repair_attempted"):
                    continue
                item = {key: row.get(key) for key in ("index", "repair_applied", "repair_attempted", "method", "before_check", "after_check")}
                for key in ("original", "revised"):
                    if row.get(key):
                        item[key] = self.persist_asset(owner, row[key], job["id"])
                comparison.append(item)
        else:
            metadata = output.get("metadata", {})
            if metadata.get("candidate_asset"):
                comparison.append({"original": self.public_asset(self.asset(owner, job["request"]["references"][0])),
                                   "revised": images[0], "candidate": self.persist_asset(owner, metadata["candidate_asset"], job["id"]),
                                   "method": metadata.get("method"), "notice": metadata.get("regional_review", {}).get("notice")})
            elif job["mode"] == "text":
                comparison.append({"original": self.public_asset(self.asset(owner, job["request"]["references"][0])), "revised": images[0], "method": "typeset"})
        return {"images": images, "comparisons": comparison, "quality": remote.get("quality", []),
                "text_quality": output.get("text_quality"), "notice": output.get("notice"),
                "inference_seconds": output.get("inference_seconds", output.get("metadata", {}).get("model_inference_seconds")),
                "workflow_seconds": output.get("workflow_seconds", remote.get("workflow_seconds"))}

    def tool(self, owner, operation, body):
        if not isinstance(body, dict):
            raise BridgeError(400, "Expected a tool request", "invalid_request")
        item = self.asset(owner, body.get("asset_id"))
        remote = dict(body, asset_id=item["remote_id"])
        if operation == "masks":
            if set(body) != {"asset_id", "png_base64"} or not isinstance(body.get("png_base64"), str) or len(body["png_base64"]) > 3 * 1024**2:
                raise BridgeError(400, "Invalid selection", "invalid_request")
            answer = self.client.request("/api/tools/masks", remote)
            ident = uuid.uuid4().hex
            mask = {"id": ident, "asset_id": item["id"], "remote_id": identifier(answer["mask_id"]), "expires_at": item["expires_at"]}
            self.write(self.owner_dir(owner) / "masks" / (ident + ".json"), mask)
            return {"id": ident}
        if operation == "ocr":
            if set(body) - {"asset_id", "expected_text"}:
                raise BridgeError(400, "Invalid OCR request", "invalid_request")
            answer = self.client.request("/api/tools/ocr", remote, timeout=85)
            answer["asset_id"] = item["id"]
            return answer
        if operation in {"preview", "text"}:
            if set(body) - {"asset_id", "layers", "parent_id"}:
                raise BridgeError(400, "Invalid text request", "invalid_request")
            remote.pop("parent_id", None)
            if operation == "preview":
                return self.client.request("/api/tools/preview", remote, image=True)
            with self.lock:
                self.quota(owner)
                parent = body.get("parent_id")
                if parent:
                    previous = self.load(owner, "jobs", parent)
                    if previous["state"] != "completed" or not any(a["id"] == item["id"] for a in previous.get("result", {}).get("images", [])):
                        raise BridgeError(400, "Previous version must supply the source image", "invalid_request")
                if not self.cpu_slots.acquire(False):
                    raise BridgeError(429, "Image tools are busy", "busy")
                ident = uuid.uuid4().hex
                job = {"id": ident, "mode": "text", "state": "running", "stage": "Typesetting", "parent_id": parent,
                       "request": {"prompt": "Text correction", "references": [item["id"]], "layers": body.get("layers")},
                       "remote_body": remote, "billing": "cpu_tool", "created_at": time.time(), "expires_at": time.time() + self.retention_days * 86400}
                try:
                    self.save_job(owner, job)
                except Exception:
                    self.cpu_slots.release()
                    raise
            threading.Thread(target=self.run_text, args=(owner, job), daemon=True).start()
            return self.public_job(job)
        raise BridgeError(404, "Unknown image tool", "not_found")

    def run_text(self, owner, job):
        try:
            self.execute(owner, job)
        except Exception:
            self.update(owner, job["id"], state="failed", stage="Text correction failed", error="Could not render the text layers; check the selection and try again.")
        finally:
            self.cpu_slots.release()

    def handle(self, method, path, body, content_type):
        self.cleanup()
        match = re.fullmatch(r"/studio/([1-9][0-9]*)/(.+)", path)
        if not match:
            raise BridgeError(404, "Unknown studio operation", "not_found")
        owner, resource = int(match[1]), match[2]
        if method == "GET" and resource == "config":
            status = {}
            for name, health in (("create", "/api/nodes/162/health"), ("edit", "/api/edit/health"), ("tools", "/api/tools/health")):
                try:
                    answer = self.client.request(health, timeout=5)
                    status[name] = answer.get("state", "unknown")
                except BridgeError:
                    status[name] = "busy" if name != "tools" and self.gpu_lock.locked() else "unavailable"
            return {"models": MODELS, "health": status, "retention_days": self.retention_days, "max_upload_mb": 10, "max_references": 3}, "application/json"
        if method == "GET" and resource == "jobs":
            rows = []
            for path in (self.owner_dir(owner) / "jobs").glob("*.json"):
                job = json.loads(path.read_text(encoding="utf-8"))
                if job["expires_at"] > time.time():
                    rows.append(self.public_job(job))
            return sorted(rows, key=lambda r: r["created_at"], reverse=True)[:100], "application/json"
        parts = resource.split("/")
        if parts[0] == "jobs" and len(parts) >= 2:
            ident = identifier(parts[1])
            if method == "GET" and len(parts) == 2:
                return self.public_job(self.load(owner, "jobs", ident)), "application/json"
            if method == "POST" and len(parts) == 3:
                if parts[2] == "authorize":
                    return self.authorize(owner, ident, body), "application/json"
                if parts[2] == "settle":
                    return self.settle(owner, ident, body), "application/json"
            if method == "DELETE" and len(parts) == 2:
                with self.lock:
                    job = self.load(owner, "jobs", ident)
                    if job["state"] not in TERMINAL | {"prepared"}:
                        raise BridgeError(409, "A running job cannot be deleted", "busy")
                    # Do not invalidate references already submitted by another job.
                    for file in (self.owner_dir(owner) / "jobs").glob("*.json"):
                        other = json.loads(file.read_text(encoding="utf-8"))
                        if other["id"] != ident and other["state"] not in TERMINAL | {"prepared"}:
                            raise BridgeError(409, "Wait for active jobs before deleting media", "busy")
                    for file in (self.owner_dir(owner) / "assets").glob("*.json"):
                        if json.loads(file.read_text(encoding="utf-8")).get("job_id") == ident:
                            file.with_suffix(".png").unlink(missing_ok=True)
                            file.unlink()
                    (self.owner_dir(owner) / "jobs" / (ident + ".json")).unlink()
                return {"deleted": True}, "application/json"
        if method == "GET" and len(parts) == 3 and parts[0] == "assets" and parts[2] == "content":
            item = self.asset(owner, parts[1])
            return (self.owner_dir(owner) / "assets" / (item["id"] + ".png")).read_bytes(), "image/png"
        if method == "POST" and resource == "uploads":
            if content_type not in {"image/png", "image/jpeg", "image/webp"} or not isinstance(body, bytes) or not 0 < len(body) <= 10 * 1024**2:
                raise BridgeError(400, "Upload a PNG, JPEG or WebP of at most 10 MB", "invalid_upload")
            self.quota(owner)
            remote = self.client.request("/api/uploads", body, content_type=content_type)
            return self.persist_asset(owner, remote), "application/json"
        if method == "POST" and resource == "jobs":
            return self.prepare(owner, body), "application/json"
        if method == "POST" and resource in {"masks", "ocr", "text", "preview"}:
            answer = self.tool(owner, resource, body)
            return answer, "image/png" if isinstance(answer, bytes) else "application/json"
        raise BridgeError(404, "Unknown studio operation", "not_found")
