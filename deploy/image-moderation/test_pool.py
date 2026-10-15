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
import threading
import time
import unittest

from pool import ReviewPool, ERROR


class PoolTests(unittest.TestCase):
    def test_thirty_completions_two_serial_replicas(self):
        active = [0, 0]
        max_active = [0, 0]
        calls = []
        gate = threading.Lock()

        def call(replica, data, timeout):
            with gate:
                active[replica] += 1
                max_active[replica] = max(max_active[replica], active[replica])
            time.sleep(0.015)
            with gate:
                active[replica] -= 1
                calls.append((replica, data))
            return b'checked:' + data

        pool = ReviewPool([0, 1], call)
        try:
            jobs = [pool.submit(str(i).encode()) for i in range(30)]
            self.assertTrue(all(jobs))
            self.assertGreater(pool.snapshot()['queued'], 0)
            for i, job in enumerate(jobs):
                self.assertTrue(job.done.wait(3))
                self.assertEqual((job.status, job.result), (200, b'checked:'+str(i).encode()))
            self.assertEqual(max_active, [1, 1])
            self.assertEqual(len(calls), 30)
            self.assertEqual(pool.snapshot()['bytes'], 0)
            self.assertEqual(pool.snapshot()['admitted'], 0)
        finally:
            pool.close()

    def test_one_down_other_drains_all_thirty(self):
        calls = []
        def call(replica, data, timeout):
            if replica == 'down':
                raise OSError('offline')
            time.sleep(0.005)
            calls.append(data)
            return data
        pool = ReviewPool(['down', 'up'], call)
        try:
            jobs = [pool.submit(str(i).encode()) for i in range(30)]
            for job in jobs:
                self.assertTrue(job.done.wait(3))
                self.assertEqual(job.status, 200)
            self.assertEqual(len(set(calls)), 30)
            self.assertEqual(pool.snapshot()['failed'], 0)
        finally:
            pool.close()

    def test_both_unavailable_fail_closed_and_drain(self):
        def call(*args):
            raise OSError('offline')
        pool = ReviewPool([0, 1], call)
        try:
            job = pool.submit(b'image')
            self.assertTrue(job.done.wait(3))
            self.assertEqual((job.status, job.result), (503, ERROR))
            self.assertEqual(pool.snapshot()['bytes'], 0)
        finally:
            pool.close()

    def test_fifo_and_capacity_are_bounded_including_active(self):
        release = threading.Event()
        started = threading.Event()
        calls = []
        def call(replica, data, timeout):
            started.set()
            release.wait(2)
            calls.append(data)
            return data
        pool = ReviewPool([0], call, capacity=3, byte_limit=9)
        try:
            first = pool.submit(b'111')
            self.assertTrue(started.wait(1))
            second = pool.submit(b'222')
            self.assertIsNone(pool.submit(b'3333'))
            third = pool.submit(b'333')
            self.assertIsNone(pool.submit(b'4'))
            release.set()
            for job in [first, second, third]:
                self.assertTrue(job.done.wait(2))
            self.assertEqual(calls, [b'111', b'222', b'333'])
        finally:
            release.set()
            pool.close()

    def test_expired_waiter_never_reaches_gpu(self):
        release = threading.Event()
        started = threading.Event()
        calls = []
        def call(replica, data, timeout):
            started.set()
            release.wait(3)
            calls.append(data)
            return data
        pool = ReviewPool([0], call, wait=1)
        try:
            first = pool.submit(b'first')
            self.assertTrue(started.wait(1))
            second = pool.submit(b'second')
            time.sleep(1.1)
            release.set()
            self.assertTrue(second.done.wait(2))
            self.assertEqual(first.status, 503)
            self.assertEqual(second.status, 503)
            self.assertEqual(calls, [b'first'])
            self.assertEqual(pool.snapshot()['bytes'], 0)
        finally:
            release.set()
            pool.close()


if __name__ == '__main__':
    unittest.main()
