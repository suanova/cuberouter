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
import hashlib
import io
import threading
from unittest.mock import Mock

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from PIL import Image

from moderation import decision_from_scores
from service import router_for


@pytest.fixture
def client(tmp_path, monkeypatch):
    key = tmp_path / 'token'
    key.write_text('t' * 40)
    monkeypatch.setenv('SHIELDGEMMA_TOKEN_FILE', str(key))
    fake = Mock(status='ready', weights_verified=True, gate=threading.Lock())
    def classify(image, data):
        return {**decision_from_scores({'sexual': .01, 'violence': .02, 'dangerous': .03}),
                'image_sha256': hashlib.sha256(data).hexdigest(),
                'model': 'google/shieldgemma-2-4b-it', 'revision': 'a' * 40}
    fake.classify.side_effect = classify
    app = FastAPI()
    app.include_router(router_for(fake))
    with TestClient(app) as client:
        yield client, fake


def png():
    out = io.BytesIO()
    Image.new('RGB', (32, 32), 'blue').save(out, format='PNG')
    return out.getvalue()


def request(client, content=None, token='t' * 40):
    return client.post('/v1/moderate', content=png() if content is None else content,
                       headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'image/png'})


def test_authentication_precedes_inference(client, monkeypatch):
    client, fake = client
    assert request(client, token='wrong').status_code == 401
    monkeypatch.delenv('SHIELDGEMMA_TOKEN_FILE')
    assert request(client).status_code == 401
    fake.classify.assert_not_called()


def test_allow_contains_exact_image_hash_and_verified_contract(client):
    client, fake = client
    result = request(client)
    assert result.status_code == 200
    data = result.json()
    assert data['protocol'] == 'image-moderation-v1'
    assert data['image_sha256'] == hashlib.sha256(png()).hexdigest()
    assert data['weights_verified'] and data['release_allowed']
    assert result.headers['cache-control'] == 'no-store'
    assert not fake.gate.locked()


def test_block_is_terminal_successful_check(client):
    client, fake = client
    fake.classify.side_effect = lambda image, data: decision_from_scores({'sexual': .99, 'violence': .02, 'dangerous': .03})
    result = request(client)
    assert result.status_code == 200
    assert result.json()['decision'] == 'block'
    assert not result.json()['release_allowed']


@pytest.mark.parametrize('status', ['awaiting_weights', 'loading', 'error'])
def test_not_ready_never_allows(client, status):
    client, fake = client
    fake.status = status
    result = request(client)
    assert result.status_code == 503
    assert not result.json()['release_allowed']
    fake.classify.assert_not_called()


def test_bad_image_or_unverified_weights_never_allow(client):
    client, fake = client
    assert request(client, b'not an image').status_code == 400
    assert request(client, b'x' * (12 * 1024 * 1024 + 1)).status_code == 400
    fake.weights_verified = False
    assert request(client).status_code == 503
    fake.classify.assert_not_called()


def test_model_error_sanitized_and_next_request_can_run(client):
    client, fake = client
    fake.classify.side_effect = RuntimeError('secret private error')
    result = request(client)
    assert result.status_code == 503
    assert 'secret' not in result.text
    assert not fake.gate.locked()
    fake.classify.side_effect = None
    fake.classify.return_value = decision_from_scores({'sexual': 0, 'violence': 0, 'dangerous': 0})
    assert request(client).json()['release_allowed']
