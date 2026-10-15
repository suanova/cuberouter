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
"""Preserve reference detail within explicitly validated C500 limits."""
import math
from contextlib import contextmanager


MAX_REFERENCE_PIXELS = 2_097_152
REFERENCE_RESOLUTION_CAPS = {
    1: math.isqrt(MAX_REFERENCE_PIXELS),
    2: math.isqrt(MAX_REFERENCE_PIXELS),
    3: 1280,
    4: 1024,
    **{count: 896 for count in range(5, 11)},
}


def reference_resolution(images):
    if not images:
        raise ValueError('missing_reference')
    if len(images) not in REFERENCE_RESOLUTION_CAPS:
        raise ValueError('too_many_references')
    # The pipeline derives an aspect-preserving, 32-pixel-aligned size from
    # this area. Single references follow their native area. Multi-reference
    # edits retain the old 1024/512 conditioning floor, so a quality upgrade
    # never reduces the existing model context for small source assets.
    # The installed pipeline takes one area budget for all references:
    # size it for the largest original, capped by
    # the validated reference count. Smaller references may be upsampled by
    # that shared pipeline setting, but never lower this budget after OOM.
    largest_area = max(image.size[0] * image.size[1] for image in images)
    floor = 256 if len(images) == 1 else (1024 if len(images) <= 3 else 512)
    return min(REFERENCE_RESOLUTION_CAPS[len(images)], max(floor, math.isqrt(largest_area)))


@contextmanager
def conditioning_cache_release(transformer):
    """Drop prefix KV tensors after the final denoising step, before VAE decode.

    The installed Qwen 2.1 pipeline otherwise keeps these local caches alive
    through decoding, although no subsequent computation reads them. Capture
    the actual request caches through a temporary PyTorch forward pre-hook;
    never mutate caches while another denoising step still needs them.
    """
    caches = []

    def capture(module, args, kwargs):
        cache = kwargs.get('kv_cache')
        if isinstance(getattr(cache, 'layer_caches', None), list):
            if all(cache is not previous for previous in caches):
                caches.append(cache)

    def clear():
        for cache in caches:
            cache.layer_caches.clear()
        caches.clear()

    def on_step_end(step, total_steps):
        if step + 1 == total_steps:
            clear()

    hook = transformer.register_forward_pre_hook(capture, with_kwargs=True)
    try:
        yield on_step_end
    finally:
        hook.remove()
        clear()
