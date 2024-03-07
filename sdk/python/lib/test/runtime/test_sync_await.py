# Copyright 2024, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import asyncio
import time
import unittest
from typing import Dict, Tuple

import pulumi
from pulumi.runtime import settings

def pulumi_test(coro):
    wrapped = pulumi.runtime.test(coro)
    def wrapper(*args, **kwargs):
        settings.configure(settings.Settings("project", "stack"))

        wrapped(*args, **kwargs)

    return wrapper


async def return_slowly(value: int, settings: Dict[str, bool]) -> Tuple[int, bool]:
    await asyncio.sleep(value * 0.001)
    return value, settings['before']


class BlockingCallTests(unittest.TestCase):
    @pulumi_test
    async def test_blocking_call_allows_async_tasks(self):
        # create a bunch of tasks numbered from 0 to 100, these will
        # each sleep (asynchronously) for that number of milliseconds
        settings = {'before':True}
        tasks = [return_slowly(i, settings) for i in range(100)]
        gather = asyncio.gather(*tasks)

        # block for 50 milliseconds, so roughly half of the tasks should have completed
        # by the time this call is completed, but most importantly they should be able to 
        # proceed while this is 'blocking'
        pulumi.blocking_call(time.sleep, 0.05)
        settings['before'] = False
        # Wait for the "slow" tasks
        results = await gather

        # verify that every task ran by using the sum of 0-99 as a crude checksum
        numbers = [el[0] for el in results]
        self.assertEqual(4950, sum(numbers))

        # roughly half of the tasks should complete before the blocking call, 
        # and roughly half after. We're checking that more than 10 and fewer than 90 did
        before = sum(1 if el[1] else 0 for el in results)
        self.assertAlmostEqual(50, before, delta=40)
    

    @pulumi_test
    async def test_blocking_thread_prevents_tasks(self):
        # create a bunch of tasks numbered from 0 to 100, these will
        # each sleep (asynchronously) for that number of milliseconds
        settings = {'before':True}
        tasks = [return_slowly(i, settings) for i in range(100)]
        gather = asyncio.gather(*tasks)

        # block for 50 milliseconds. None of the async tasks will be
        # able to proceed while this is happening
        time.sleep(0.05)
        settings['before'] = False

        # wait for the "slow" tasks
        results = await gather

        # verify that every task ran by using the sum of 0-99 as a crude checksum
        numbers = [el[0] for el in results]
        self.assertEqual(4950, sum(numbers))

        # zero, of the tasks will have been scheduled before the blocking call to 
        # time.sleep because the thread never yielded.
        before = sum(1 if el[1] else 0 for el in results)
        self.assertEqual(0, before)
    

