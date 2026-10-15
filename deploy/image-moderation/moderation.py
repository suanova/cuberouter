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
"""Local image-only ShieldGemma 2 proof of concept; no image-generation path."""
from __future__ import annotations

import hashlib
import io
import math
import threading
import time
import warnings
from pathlib import Path

from PIL import Image, ImageOps, UnidentifiedImageError

MODEL_ID = "google/shieldgemma-2-4b-it"
POLICIES = ("sexual", "violence", "dangerous")
MAX_BYTES = 12 * 1024 * 1024
MAX_PIXELS = 20_000_000
THRESHOLD = 0.5  # PoC threshold, NOT calibrated for production.
Image.MAX_IMAGE_PIXELS = MAX_PIXELS


class InvalidImage(ValueError):
    pass


def decode_image(data: bytes) -> Image.Image:
    if not data or len(data) > MAX_BYTES:
        raise InvalidImage("Image must be between 1 byte and 12 MiB.")
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(data)) as image:
                if image.format not in {"JPEG", "PNG", "WEBP"}:
                    raise InvalidImage("Only JPEG, PNG and WebP are supported.")
                if getattr(image, "n_frames", 1) != 1:
                    raise InvalidImage("Animated images are not supported.")
                if image.width * image.height > MAX_PIXELS:
                    raise InvalidImage("Image exceeds 20 megapixels.")
                image.load()
                oriented = ImageOps.exif_transpose(image)
                rgba = oriented.convert("RGBA")
                background = Image.new("RGBA", rgba.size, "white")
                background.alpha_composite(rgba)
                return background.convert("RGB")
    except (UnidentifiedImageError, OSError, Image.DecompressionBombError,
            Image.DecompressionBombWarning) as exc:
        raise InvalidImage("Invalid or oversized image.") from exc


def decision_from_scores(scores: dict[str, float], threshold: float = THRESHOLD) -> dict:
    if not math.isfinite(threshold) or not 0 < threshold < 1:
        raise ValueError("Invalid moderation threshold.")
    if set(scores) != set(POLICIES):
        raise ValueError("All three image policies must be checked.")
    if any(not math.isfinite(v) or not 0 <= v <= 1 for v in scores.values()):
        raise ValueError("Invalid model probabilities.")
    violations = [policy for policy in POLICIES if scores[policy] >= threshold]
    return {
        "decision": "block" if violations else "allow",
        "release_allowed": not violations,
        "violations": violations,
        "scores": scores,
        "threshold": threshold,
        "threshold_calibrated": False,
    }


class ShieldRuntime:
    def __init__(self, model_dir: Path):
        self.model_dir = model_dir
        self.status = "not_loaded"
        self.error = None
        self.model = None
        self.processor = None
        self.device_name = None
        self.revision = None
        self.weights_verified = False
        self.gate = threading.Lock()
        self._loader_lock = threading.Lock()

    def health(self) -> dict:
        return {
            "model": MODEL_ID,
            "status": self.status,
            "ready": self.status == "ready",
            "error": self.error,
            "device": self.device_name,
            "revision": self.revision,
            "weights_verified": self.weights_verified,
            "policies": list(POLICIES),
            "threshold": THRESHOLD,
            "threshold_calibrated": False,
            "input_type": "image_only",
            "production_connected": False,
            "stores_uploaded_images": False,
            "busy": self.gate.locked(),
        }

    def load(self) -> None:
        if not self._loader_lock.acquire(blocking=False):
            return
        try:
            if self.status == "ready":
                return
            if not (self.model_dir / "model.safetensors.index.json").exists() and not (self.model_dir / "model.safetensors").exists():
                self.status = "awaiting_weights"
                self.error = "Official model weights are not downloaded. Run Login-And-Download.ps1."
                return
            self.status = "loading"
            self.error = None
            import json
            import torch
            from transformers import AutoProcessor, ShieldGemma2ForImageClassification

            if not torch.cuda.is_available():
                raise RuntimeError("CUDA GPU unavailable")
            free, _ = torch.cuda.mem_get_info()
            if free < 12 * 1024**3:
                raise RuntimeError("At least 12 GiB free GPU memory is required for this PoC")
            self.device_name = torch.cuda.get_device_name(0)
            metadata = self.model_dir / "download-metadata.json"
            if metadata.exists():
                self.revision = json.loads(metadata.read_text(encoding="utf-8"))["revision"]
            processor = AutoProcessor.from_pretrained(self.model_dir, local_files_only=True)
            model, loading_info = ShieldGemma2ForImageClassification.from_pretrained(
                self.model_dir,
                torch_dtype=torch.bfloat16,
                device_map={"": "cuda:0"},
                # Transformers 4.57.6's ShieldGemma2 wrapper rejects SDPA.
                attn_implementation="eager",
                local_files_only=True,
                output_loading_info=True,
            )
            # In Transformers 4.57.6 the wrapper ties the text backbone, not
            # Gemma3's output head. The checkpoint intentionally omits that
            # shared head; leaving its random initialization gives bogus scores.
            # https://huggingface.co/google/shieldgemma-2-4b-it/discussions/10
            missing = set(loading_info.get("missing_keys", []))
            if (missing - {"model.lm_head.weight"} or
                    loading_info.get("unexpected_keys") or
                    loading_info.get("mismatched_keys") or
                    loading_info.get("error_msgs")):
                raise RuntimeError("Checkpoint weights did not load completely")
            if not model.config.text_config.tie_word_embeddings:
                raise RuntimeError("Expected official checkpoint with tied embeddings")
            model.model.tie_weights()
            if model.model.get_output_embeddings().weight is not model.model.get_input_embeddings().weight:
                raise RuntimeError("Classifier output head is not tied to checkpoint embeddings")
            model.eval()
            self.model, self.processor = model, processor
            self.weights_verified = True
            self.status = "ready"
        except Exception as exc:
            self.status = "error"
            # Never return tracebacks, tokens, or arbitrary upstream messages.
            self.error = "Model load failed: " + type(exc).__name__
        finally:
            self._loader_lock.release()

    def classify(self, image: Image.Image, original_bytes: bytes) -> dict:
        import torch

        if self.status != "ready":
            raise RuntimeError("Model is not ready")
        start = time.perf_counter()
        torch.cuda.reset_peak_memory_stats()
        scores = {}
        with torch.inference_mode():
            # Sequential policies reduce peak GPU memory on the classifier device.
            for policy in POLICIES:
                inputs = self.processor(images=[image], policies=[policy], return_tensors="pt")
                inputs = inputs.to(device="cuda:0", dtype=torch.bfloat16)
                output = self.model(**inputs, use_cache=False, logits_to_keep=1)
                # Implementation selects [yes_token_index, no_token_index].
                # Column 0 means Yes = policy violation; never use column 1 here.
                probability = output.probabilities
                if tuple(probability.shape) != (1, 2):
                    raise RuntimeError("Unexpected classification output")
                scores[policy] = float(probability[0, 0].float().cpu())
                del inputs, output, probability
        torch.cuda.synchronize()
        return {
            **decision_from_scores(scores),
            "model": MODEL_ID,
            "revision": self.revision,
            "image_sha256": hashlib.sha256(original_bytes).hexdigest(),
            "image_size": list(image.size),
            "latency_seconds": round(time.perf_counter() - start, 3),
            "peak_allocated_gib": round(torch.cuda.max_memory_allocated() / 1024**3, 3),
            "input_type": "image_only",
        }
