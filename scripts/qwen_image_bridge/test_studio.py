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

import copy
import json
import struct
import tempfile
import time
import unittest
from pathlib import Path
from bridge import BridgeError, PNG_MAGIC
from studio import Studio


class Worker:
    deadline = 2
    poll_seconds = 0

    def __init__(self):
        self.calls = []

    def request(self, path, body=None, image=False, **kwargs):
        self.calls.append((path, body))
        if image:
            return PNG_MAGIC + b'\0\0\0\rIHDR' + struct.pack('>II', 1328, 1328)
        if path == '/api/workflow/generate':
            return {'job_id': 'b' * 32}
        if path == '/api/workflow/jobs/' + 'b' * 32:
            return {'job_id': 'b' * 32, 'state': 'completed', 'result': {'images': [{'asset_id': 'a' * 32, 'image_url': '/media/' + 'a' * 32 + '.png'}]}}
        if path.startswith('/api/workflow/comparison/'):
            return {'images': []}
        raise AssertionError(path)


class StudioTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.worker = Worker()
        self.studio = Studio(self.directory.name, self.worker)
        self.request = {'mode': 'create', 'prompt': 'An orange cat, no text', 'size': '1328x1328', 'steps': 20, 'seed': 0, 'cfg': 0, 'count': 1}

    def prepared(self):
        return self.studio.prepare(7, self.request)

    def complete(self):
        prepared = self.prepared()
        self.studio.authorize(7, prepared['job']['id'], prepared['relay_body'])
        result = self.studio.run_relay(prepared['relay_body'])
        self.studio.settle(7, prepared['job']['id'], {'success': True, 'request_id': 'usage-123'})
        return prepared, result

    def test_result_is_private_until_relay_settles_and_count_is_preserved(self):
        prepared = self.prepared(); ident = prepared['job']['id']
        self.studio.authorize(7, ident, prepared['relay_body'])
        result = self.studio.run_relay(prepared['relay_body'])
        job = self.studio.load(7, 'jobs', ident); asset = job['result']['images'][0]['id']
        self.assertNotIn('result', self.studio.public_job(job))
        with self.assertRaises(BridgeError): self.studio.asset(7, asset)
        self.assertEqual(len(result['data']), 1)
        self.studio.settle(7, ident, {'success': True, 'request_id': 'usage-123'})
        self.assertEqual(self.studio.asset(7, asset)['id'], asset)
        self.assertEqual(self.studio.load(7, 'jobs', ident)['request_id'], 'usage-123')
        public = json.dumps(self.studio.public_job(self.studio.load(7, 'jobs', ident)))
        self.assertNotIn('studio_token', public)
        self.assertNotIn('remote_id', public)
        self.assertNotIn('/media/', public)

    def test_another_account_cannot_read_job_asset_or_reuse_reference(self):
        prepared, _ = self.complete(); ident = prepared['job']['id']
        asset = self.studio.load(7, 'jobs', ident)['result']['images'][0]['id']
        for action in (lambda: self.studio.load(8, 'jobs', ident), lambda: self.studio.asset(8, asset), lambda: self.studio.prepare(8, dict(self.request, mode='edit', size='auto', references=[asset]))):
            with self.assertRaises(BridgeError) as caught: action()
            self.assertEqual(caught.exception.status, 404)

    def test_modified_or_replayed_ticket_never_runs_a_second_generation(self):
        prepared = self.prepared()
        changed = copy.deepcopy(prepared['relay_body']); changed['n'] = 4
        with self.assertRaises(BridgeError): self.studio.authorize(7, prepared['job']['id'], changed)
        with self.assertRaises(BridgeError): self.studio.run_relay(prepared['relay_body'])
        self.studio.authorize(7, prepared['job']['id'], prepared['relay_body'])
        self.studio.run_relay(prepared['relay_body'])
        with self.assertRaises(BridgeError): self.studio.run_relay(prepared['relay_body'])
        self.assertEqual(sum(path == '/api/workflow/generate' for path, _ in self.worker.calls), 1)

    def test_failed_relay_keeps_worker_output_unreleased(self):
        prepared = self.prepared(); ident = prepared['job']['id']
        self.studio.authorize(7, ident, prepared['relay_body']); self.studio.run_relay(prepared['relay_body'])
        self.studio.settle(7, ident, {'success': False})
        job = self.studio.load(7, 'jobs', ident)
        self.assertEqual(job['billing'], 'relay_failed')
        self.assertNotIn('result', self.studio.public_job(job))
        with self.assertRaises(BridgeError): self.studio.asset(7, job['result']['images'][0]['id'])

    def test_late_gpu_completion_cannot_undo_failed_relay_settlement(self):
        prepared = self.prepared(); ident = prepared['job']['id']
        self.studio.authorize(7, ident, prepared['relay_body'])
        request = self.worker.request
        def after_timeout(path, *args, **kwargs):
            if path == '/api/workflow/jobs/' + 'b' * 32:
                self.studio.settle(7, ident, {'success': False, 'request_id': 'timed-out'})
            return request(path, *args, **kwargs)
        self.worker.request = after_timeout
        with self.assertRaises(BridgeError): self.studio.run_relay(prepared['relay_body'])
        job = self.studio.load(7, 'jobs', ident)
        self.assertEqual(job['state'], 'failed')
        self.assertEqual(job['billing'], 'relay_failed')
        self.assertNotIn('result', self.studio.public_job(job))

    def test_wrong_image_dimensions_are_rejected_before_release(self):
        prepared = self.studio.prepare(7, dict(self.request, size='1664x928'))
        self.studio.authorize(7, prepared['job']['id'], prepared['relay_body'])
        with self.assertRaises(BridgeError): self.studio.run_relay(prepared['relay_body'])

    def test_invalid_text_parent_does_not_exhaust_cpu_slots(self):
        prepared, _ = self.complete()
        asset = self.studio.load(7, 'jobs', prepared['job']['id'])['result']['images'][0]['id']
        for _ in range(3):
            with self.assertRaises(BridgeError) as caught:
                self.studio.tool(7, 'text', {'asset_id': asset, 'parent_id': 'c' * 32, 'layers': []})
            self.assertEqual(caught.exception.status, 404)
        self.assertTrue(self.studio.cpu_slots.acquire(False))
        self.assertTrue(self.studio.cpu_slots.acquire(False))
        self.studio.cpu_slots.release(); self.studio.cpu_slots.release()

    def test_restart_marks_inflight_job_interrupted_without_resubmitting(self):
        prepared = self.prepared(); self.studio.authorize(7, prepared['job']['id'], prepared['relay_body'])
        restarted = Studio(self.directory.name, self.worker)
        self.assertEqual(restarted.load(7, 'jobs', prepared['job']['id'])['state'], 'interrupted')
        self.assertEqual(self.worker.calls, [])

    def test_high_steps_cannot_choose_a_cheap_tier_and_zero_values_are_retained(self):
        result = self.studio.prepare(7, dict(self.request, steps=50))
        self.assertEqual(result['relay_body']['quality'], 'high')
        self.assertEqual(result['relay_body']['seed'], 0)
        self.assertEqual(result['relay_body']['true_cfg_scale'], 0)
        with self.assertRaises(BridgeError): self.studio.prepare(8, dict(self.request, count=1000000000000))
        self.assertEqual(self.worker.calls, [])

    def test_replacing_an_unsubmitted_preparation_does_not_block_the_user(self):
        old = self.prepared()
        new = self.prepared()
        self.assertNotEqual(old['job']['id'], new['job']['id'])
        with self.assertRaises(BridgeError): self.studio.authorize(7, old['job']['id'], old['relay_body'])
        self.studio.authorize(7, new['job']['id'], new['relay_body'])

    def test_deleting_saved_version_removes_its_bytes_and_history(self):
        prepared, _ = self.complete(); ident = prepared['job']['id']
        asset = self.studio.load(7, 'jobs', ident)['result']['images'][0]['id']
        self.studio.handle('DELETE', f'/studio/7/jobs/{ident}', None, '')
        self.assertFalse((Path(self.directory.name) / '7/assets' / (asset + '.png')).exists())
        with self.assertRaises(BridgeError): self.studio.load(7, 'jobs', ident)

    def test_expired_asset_is_not_served_and_cleanup_removes_files(self):
        prepared, _ = self.complete()
        asset = self.studio.load(7, 'jobs', prepared['job']['id'])['result']['images'][0]['id']
        path = Path(self.directory.name) / '7/assets' / (asset + '.json')
        record = json.loads(path.read_text()); record['expires_at'] = time.time() - 1
        self.studio.write(path, record)
        with self.assertRaises(BridgeError) as caught: self.studio.asset(7, asset)
        self.assertEqual(caught.exception.status, 410)
        self.studio.cleanup()
        self.assertFalse(path.with_suffix('.png').exists())


if __name__ == '__main__': unittest.main()
