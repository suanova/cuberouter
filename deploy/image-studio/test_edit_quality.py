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
import math
import unittest
from types import SimpleNamespace

from edit_quality import (MAX_REFERENCE_PIXELS, REFERENCE_RESOLUTION_CAPS,
                          conditioning_cache_release, reference_resolution)


def image(width, height):
    return SimpleNamespace(size=(width, height))


class EditQualityTests(unittest.TestCase):
    def test_native_single_reference_retains_supported_dimensions(self):
        # Match the pipeline's area/aspect -> 32-pixel bucket calculation.
        for width, height in ((1280, 1280), (1312, 1312), (1536, 1024),
                              (1664, 928), (672, 640), (512, 512)):
            with self.subTest(size=(width, height)):
                side = reference_resolution([image(width, height)])
                calculated_width = round(math.sqrt(side * side * width / height) / 32) * 32
                calculated_height = round(math.sqrt(side * side * height / width) / 32) * 32
                self.assertEqual((calculated_width, calculated_height), (width, height))

    def test_large_single_reference_is_bounded(self):
        self.assertEqual(reference_resolution([image(4096, 4096)]), math.isqrt(MAX_REFERENCE_PIXELS))

    def test_small_reference_meets_vision_minimum(self):
        self.assertEqual(reference_resolution([image(64, 64)]), 256)

    def test_two_native_references_are_not_always_shrunk_to_1024(self):
        self.assertEqual(reference_resolution([image(1312, 1312)] * 2), 1312)

    def test_four_large_references_no_longer_use_the_ten_image_cap(self):
        self.assertEqual(reference_resolution([image(1312, 1312)] * 4), 1024)

    def test_three_native_1280_references_keep_their_area(self):
        self.assertEqual(reference_resolution([image(1280, 1280)] * 3), 1280)

    def test_ten_large_references_use_the_higher_quality_cap(self):
        self.assertEqual(reference_resolution([image(1312, 1312)] * 10), 896)

    def test_small_sources_keep_the_previous_conditioning_floor(self):
        for count in range(1, 11):
            with self.subTest(count=count):
                expected = 384 if count == 1 else (1024 if count <= 3 else 512)
                self.assertEqual(reference_resolution([image(384, 384)] * count), expected)

    def test_mixed_references_use_largest_source_area_and_are_order_independent(self):
        sources = [image(384, 384), image(1312, 1312)]
        self.assertEqual(reference_resolution(sources), 1312)
        self.assertEqual(reference_resolution(list(reversed(sources))), 1312)

    def test_large_references_obey_validated_count_caps(self):
        for count, cap in REFERENCE_RESOLUTION_CAPS.items():
            with self.subTest(count=count):
                self.assertEqual(reference_resolution([image(4096, 4096)] * count), cap)

    def test_more_than_ten_references_are_rejected(self):
        with self.assertRaises(ValueError):
            reference_resolution([image(512, 512)] * 11)

    def test_missing_reference_is_rejected(self):
        with self.assertRaises(ValueError):
            reference_resolution([])


class Transformer:
    def __init__(self):
        self.hooks = []

    def register_forward_pre_hook(self, callback, *, with_kwargs):
        assert with_kwargs
        self.hooks.append(callback)
        return SimpleNamespace(remove=lambda: self.hooks.remove(callback))

    def forward(self, cache):
        for callback in self.hooks:
            callback(self, (), {'kv_cache': cache})


class ConditioningCacheTests(unittest.TestCase):
    def test_caches_survive_until_final_step_and_both_cfg_branches_are_released(self):
        transformer = Transformer()
        positive = SimpleNamespace(layer_caches=[object()])
        negative = SimpleNamespace(layer_caches=[object()])
        with conditioning_cache_release(transformer) as progress:
            transformer.forward(positive)
            transformer.forward(negative)
            transformer.forward(positive)
            progress(0, 2)
            self.assertEqual(len(positive.layer_caches), 1)
            self.assertEqual(len(negative.layer_caches), 1)
            progress(1, 2)
            self.assertEqual(positive.layer_caches, [])
            self.assertEqual(negative.layer_caches, [])
        self.assertEqual(transformer.hooks, [])

    def test_failure_removes_hook_and_releases_cached_references(self):
        transformer = Transformer()
        cache = SimpleNamespace(layer_caches=[object()])
        with self.assertRaises(RuntimeError):
            with conditioning_cache_release(transformer):
                transformer.forward(cache)
                raise RuntimeError('inference failed')
        self.assertEqual(transformer.hooks, [])
        self.assertEqual(cache.layer_caches, [])

    def test_requests_do_not_share_cache_tracking(self):
        transformer = Transformer()
        old = SimpleNamespace(layer_caches=[object()])
        with conditioning_cache_release(transformer) as progress:
            transformer.forward(old)
            progress(0, 1)
        old.layer_caches.append('outside the finished request')
        new = SimpleNamespace(layer_caches=[object()])
        with conditioning_cache_release(transformer) as progress:
            transformer.forward(new)
            progress(0, 1)
        self.assertEqual(old.layer_caches, ['outside the finished request'])
        self.assertEqual(new.layer_caches, [])

    def test_disabled_kv_cache_is_supported(self):
        transformer = Transformer()
        with conditioning_cache_release(transformer) as progress:
            transformer.forward(None)
            progress(0, 1)
        self.assertEqual(transformer.hooks, [])


if __name__ == '__main__':
    unittest.main()
