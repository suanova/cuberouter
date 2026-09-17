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

import base64
import struct
import threading
import unittest
from unittest.mock import Mock, patch
from urllib.error import HTTPError
from urllib.request import Request, urlopen

from bridge import BridgeError, BridgeServer, MODEL, PNG_MAGIC, WorkflowClient, to_workflow

JOB = "a" * 32
ASSET = "b" * 32


class RequestContractTests(unittest.TestCase):
    def test_batch_ratio_quality_and_explicit_zero_values_reach_workflow(self):
        request = {"model": MODEL, "prompt": "  a cat  ", "n": 4, "size": "1472x1104",
                   "quality": "fast", "seed": 0, "true_cfg_scale": 0}
        self.assertEqual(to_workflow(request), {"mode": "create", "parameters": {
            "prompt": "a cat", "aspect_ratio": "4:3", "num_images_per_prompt": 4,
            "num_inference_steps": 20, "seed": 0, "true_cfg_scale": 0}})

    def test_invalid_parameters_are_rejected_before_submission(self):
        invalid = [{"n": 0}, {"n": 5}, {"n": True}, {"n": 2**64}, {"seed": -1},
                   {"seed": 2**63, "n": 4}, {"size": "1024x1024"}, {"size": []},
                   {"true_cfg_scale": float("nan")}, {"true_cfg_scale": True},
                   {"num_inference_steps": 101}, {"quality": "ultra"}, {"prompt": " "},
                   {"prompt": "x" * 16001}, {"model": "other"}, {"image": "http://example.test"},
                   {"response_format": "url"}]
        for change in invalid:
            with self.subTest(change=repr(change)[:80]), self.assertRaises(BridgeError) as caught:
                to_workflow({"model": MODEL, "prompt": "a cat", **change})
            self.assertEqual(caught.exception.status, 400)


class WorkflowTests(unittest.TestCase):
    def setUp(self):
        self.client = WorkflowClient("http://127.0.0.1:18162", poll_seconds=0)
        self.body = {"model": MODEL, "prompt": "a cat", "size": "1328x1328"}
        self.png = PNG_MAGIC + struct.pack(">I", 13) + b"IHDR" + struct.pack(">II", 1328, 1328)
        self.completed = {"job_id": JOB, "state": "completed", "quality": [{"state": "informational"}],
                          "result": {"images": [{"image_url": "/media/" + ASSET + ".png"}],
                                     "text_quality": "informational", "inference_seconds": 12}}

    def test_submission_waits_for_completion_and_preserves_ocr_advisory(self):
        self.client.request = Mock(side_effect=[{"job_id": JOB}, {"job_id": JOB, "state": "running"},
                                               self.completed, self.png])
        response = self.client.generate(self.body)
        self.assertEqual(base64.b64decode(response["data"][0]["b64_json"]), self.png)
        self.assertEqual(response["workflow"]["text_quality"], "informational")
        self.assertEqual(response["workflow"]["quality"], [{"state": "informational"}])
        self.assertEqual(self.client.request.call_args_list[0].args[0], "/api/workflow/generate")

    def test_failed_or_held_job_never_downloads_an_image(self):
        for state in ("failed", "needs_review"):
            self.client.request = Mock(side_effect=[{"job_id": JOB}, {"job_id": JOB, "state": state}])
            with self.subTest(state=state), self.assertRaises(BridgeError) as caught:
                self.client.generate(self.body)
            self.assertEqual(caught.exception.status, 422)
            self.assertEqual(self.client.request.call_count, 2)

    def test_wrong_job_identity_is_rejected(self):
        self.client.request = Mock(side_effect=[{"job_id": JOB}, {**self.completed, "job_id": ASSET}])
        with self.assertRaises(BridgeError):
            self.client.generate(self.body)

    def test_unexpected_image_count_is_rejected(self):
        with self.assertRaises(BridgeError):
            self.client.to_openai(self.completed, {**self.body, "n": 2})

    def test_external_or_unreleased_asset_paths_are_never_fetched(self):
        for path in ("http://example.test/x.png", "/media/../secret", "/images/" + ASSET + ".png"):
            self.client.request = Mock()
            job = {**self.completed, "result": {"images": [{"image_url": path}]}}
            with self.subTest(path=path), self.assertRaises(BridgeError):
                self.client.to_openai(job, self.body)
            self.client.request.assert_not_called()

    def test_non_png_and_wrong_dimensions_are_rejected(self):
        for raw in (b"<html>not an image</html>", self.png[:16] + struct.pack(">II", 512, 512)):
            self.client.request = Mock(return_value=raw)
            with self.subTest(raw=raw), self.assertRaises(BridgeError):
                self.client.to_openai(self.completed, self.body)

    def test_timeout_does_not_resubmit_the_generation(self):
        self.client.request = Mock(return_value={"job_id": JOB})
        with patch("bridge.time.monotonic", side_effect=[0, 541]), self.assertRaises(BridgeError) as caught:
            self.client.generate(self.body)
        self.assertEqual(caught.exception.code, "workflow_pending")
        self.client.request.assert_called_once()


class HttpBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.client = Mock()
        self.key = "a-local-test-key-with-enough-characters"
        self.server = BridgeServer(("127.0.0.1", 0), self.client, self.key)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.url = "http://127.0.0.1:" + str(self.server.server_port)

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def test_missing_key_cannot_start_generation(self):
        request = Request(self.url + "/v1/images/generations", data=b"{}",
                          headers={"Content-Type": "application/json"})
        with self.assertRaises(HTTPError) as caught:
            urlopen(request)
        self.assertEqual(caught.exception.code, 401)
        self.client.generate.assert_not_called()

    def test_busy_adapter_rejects_second_request_without_submission(self):
        self.server.generation_lock.acquire()
        self.addCleanup(self.server.generation_lock.release)
        request = Request(self.url + "/v1/images/generations",
                          data=b'{"model":"qwen-image-2512","prompt":"a cat"}',
                          headers={"Authorization": "Bearer " + self.key, "Content-Type": "application/json"})
        with self.assertRaises(HTTPError) as caught:
            urlopen(request)
        self.assertEqual(caught.exception.code, 429)
        self.client.generate.assert_not_called()


if __name__ == "__main__":
    unittest.main()
