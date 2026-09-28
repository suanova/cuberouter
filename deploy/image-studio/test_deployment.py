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
import os
from pathlib import Path
import tempfile
import unittest

from configure_deployment import channel_ids, load_tokens, make_overlay, private_write


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.base = {'services': {'gateway': {
            'image': 'existing:tag', 'environment': {'SQL_DSN': 'existing-db', 'SESSION_SECRET': 'unchanged'},
            'volumes': ['existing-data:/data'], 'ports': ['13000:3000'],
            'networks': {'existing-private': {'aliases': ['cr']}}
        }, 'database': {'image': 'postgres:15'}}, 'networks': {'existing-private': {}}}
        self.tokens = {'dispatcher_token': 'd' * 48, 'relay_token': 'r' * 48}

    def test_overlay_connects_both_directions_without_replacing_database_or_network(self):
        before = copy.deepcopy(self.base)
        overlay = make_overlay(self.base, 'gateway', channel_ids('21,22,23,24,25,26,27,28'), self.tokens, 3000)
        self.assertEqual(self.base, before)
        gateway = overlay['services']['gateway']
        dispatcher = overlay['services']['image-studio-dispatcher']
        self.assertEqual(set(gateway), {'networks', 'environment'})
        self.assertEqual(gateway['networks']['existing-private'], {'aliases': ['cr']})
        env = gateway['environment']
        self.assertEqual(env['IMAGE_STUDIO_DISPATCHER_URL'], 'http://image-studio-dispatcher:18195')
        self.assertEqual(env['IMAGE_STUDIO_AUDIT_ENABLED'], 'true')
        self.assertEqual(env['IMAGE_STUDIO_ALLOWED_GROUPS'], 'image-studio')
        self.assertEqual(env['IMAGE_STUDIO_CHANNEL_IDS'], '21,22,23,24,25,26,27,28')
        other = dispatcher['environment']
        self.assertEqual(other['IMAGE_STUDIO_RELAY_URL'], 'http://gateway:3000')
        self.assertEqual(other['IMAGE_STUDIO_RELAY_CONCURRENCY'], '8')
        for key in ('IMAGE_STUDIO_DISPATCHER_TOKEN', 'IMAGE_STUDIO_RELAY_TOKEN'):
            self.assertEqual(env[key], other[key])
        self.assertNotIn('ports', dispatcher)
        self.assertNotIn('database', overlay['services'])
        self.assertTrue(overlay['networks']['image-studio-private']['internal'])

    def test_default_network_and_released_image(self):
        del self.base['services']['gateway']['networks']
        o = make_overlay(self.base, 'gateway', ['31'], self.tokens, 8080, 'registry/studio:v1')
        self.assertIn('default', o['services']['gateway']['networks'])
        dispatcher = o['services']['image-studio-dispatcher']
        self.assertEqual(dispatcher['image'], 'registry/studio:v1')
        self.assertNotIn('build', dispatcher)
        self.assertEqual(dispatcher['environment']['IMAGE_STUDIO_RELAY_CONCURRENCY'], '1')

    def test_rejects_ambiguous_or_unsupported_capacity(self):
        for ids in ('', '0', '1,1', '1,', '1,2,3,4,5,6,7,8,9', '1;2'):
            with self.subTest(ids=ids), self.assertRaises(ValueError):
                channel_ids(ids)
        self.assertEqual(channel_ids(' 2, 3 '), ['2', '3'])
        for change in ({'deploy': {'replicas': 2}}, {'scale': 2}, {'network_mode': 'host'}):
            base = copy.deepcopy(self.base)
            base['services']['gateway'].update(change)
            with self.assertRaises(ValueError):
                make_overlay(base, 'gateway', ['1'], self.tokens, 3000)

    def test_rejects_existing_dispatcher_and_shared_secret(self):
        self.base['services']['image-studio-dispatcher'] = {}
        with self.assertRaises(ValueError):
            make_overlay(self.base, 'gateway', ['1'], self.tokens, 3000)
        del self.base['services']['image-studio-dispatcher']
        self.tokens['relay_token'] = self.tokens['dispatcher_token']
        with self.assertRaises(ValueError):
            make_overlay(self.base, 'gateway', ['1'], self.tokens, 3000)

    def test_rerun_keeps_internal_secrets_and_refuses_cross_project_reuse(self):
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory)
            a = load_tokens(folder, 'existing-project')
            b = load_tokens(folder, 'existing-project')
            self.assertEqual(a, b)
            self.assertGreaterEqual(len(a['dispatcher_token']), 32)
            self.assertNotEqual(a['dispatcher_token'], a['relay_token'])
            with self.assertRaises(ValueError):
                load_tokens(folder, 'different-project')
            output = folder / 'overlay.json'
            private_write(output, {'secret': 'test'})
            self.assertEqual(json.loads(output.read_text()), {'secret': 'test'})
            if os.name == 'posix':
                self.assertEqual(output.stat().st_mode & 0o777, 0o600)


if __name__ == '__main__':
    unittest.main()
