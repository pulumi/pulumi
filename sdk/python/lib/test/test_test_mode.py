# Copyright 2016-2018, Pulumi Corporation.
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
import unittest

from pulumi import Output
from pulumi.errors import RunError
from pulumi.resource import CustomResource
from pulumi.runtime.settings import _set_project, _set_stack, _set_test_mode_enabled, get_project, get_stack


class FakeResource(CustomResource):
    x: Output[float]

    def __init__(__self__, name, x=None):
        __props__ = dict()
        __props__['x'] = x
        super(FakeResource, __self__).__init__('python:test:FakeResource', name, __props__, None)


def async_test(coro):
    def wrapper(*args, **kwargs):
        loop = asyncio.new_event_loop()
        loop.run_until_complete(coro(*args, **kwargs))
        loop.close()
    return wrapper


class TestModeTests(unittest.TestCase):
    def test_reject_non_test_resource(self):
        self.assertRaises(RunError, lambda: FakeResource("fake"))

    def test_reject_non_test_project(self):
        self.assertRaises(RunError, lambda: get_project())

    def test_reject_non_test_stack(self):
        self.assertRaises(RunError, lambda: get_stack())

    @async_test
    async def test_test_mode_values(self):
        # Swap in temporary values.
        _set_test_mode_enabled(True)
        test_project = "TestProject"
        _set_project(test_project)
        test_stack = "TestStack"
        _set_stack(test_stack)
        try:
            # Now access the settings -- in test mode, this will work.
            p = get_project()
            self.assertEqual(test_project, p)
            s = get_stack()
            self.assertEqual(test_stack, s)

            # Allocate a resource and make sure its output property is set as expected.
            x_fut = asyncio.Future()
            res = FakeResource("fake", x=42)
            res.x.apply(lambda x: x_fut.set_result(x))
            x_val = await x_fut
            self.assertEqual(42, x_val)
        finally:
            # Reset global state back to its previous settings.
            _set_test_mode_enabled(False)
            _set_project(None)
            _set_stack(None)
