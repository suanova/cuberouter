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
import io
import threading
from unittest.mock import Mock

import pytest
from fastapi.testclient import TestClient
from PIL import Image

from moderation import InvalidImage, decision_from_scores, decode_image


def png():
    buffer = io.BytesIO()
    Image.new("RGB", (16, 16), "blue").save(buffer, format="PNG")
    return buffer.getvalue()


@pytest.mark.parametrize("policy", ["sexual", "violence", "dangerous"])
def test_any_policy_blocks_at_threshold(policy):
    scores = {"sexual": 0.01, "violence": 0.01, "dangerous": 0.01}
    scores[policy] = 0.5
    result = decision_from_scores(scores)
    assert result["release_allowed"] is False
    assert result["violations"] == [policy]


@pytest.mark.parametrize("scores", [
    {"sexual": 0.01},
    {"sexual": float("nan"), "violence": 0.01, "dangerous": 0.01},
    {"sexual": -0.1, "violence": 0.01, "dangerous": 0.01},
])
def test_invalid_or_incomplete_scores_never_allow(scores):
    with pytest.raises(ValueError):
        decision_from_scores(scores)


def test_clean_scores_allow():
    assert decision_from_scores({"sexual": 0.01, "violence": 0.02, "dangerous": 0.03})["release_allowed"]


def test_decode_rejects_non_image():
    with pytest.raises(InvalidImage):
        decode_image(b"this is a prompt, not an image")
