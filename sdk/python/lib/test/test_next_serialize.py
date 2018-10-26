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

from pulumi.next.resource import CustomResource
from pulumi.next.runtime import rpc, known_types
from pulumi.next.output import Output
from pulumi.next.asset import FileAsset, RemoteAsset, StringAsset


class FakeCustomResource:
    """
    Fake CustomResource class that duck-types to the real CustomResource.
    This class is substituted for the real CustomResource for the below test.
    """
    def __init__(self, id):
        self.id = id


def async_test(coro):
    def wrapper(*args, **kwargs):
        loop = asyncio.new_event_loop()
        return loop.run_until_complete(coro(*args, **kwargs))
    return wrapper


class NextSerializationTests(unittest.TestCase):
    def setUp(self):
        known_types._custom_resource_type = FakeCustomResource

    def tearDown(self):
        known_types._custom_resource_type = CustomResource

    @async_test
    async def test_list(self):
        test_list = [1, 2, 3]
        props = await rpc.serialize_property(test_list, [])
        self.assertEqual(test_list, props)

    @async_test
    async def test_future(self):
        fut = asyncio.Future()
        fut.set_result(42)
        prop = await rpc.serialize_property(fut, [])
        self.assertEqual(42, prop)

    @async_test
    async def test_coro(self):
        async def fun():
            await asyncio.sleep(0.1)
            return 42

        prop = await rpc.serialize_property(fun(), [])
        self.assertEqual(42, prop)

    @async_test
    async def test_dict(self):
        fut = asyncio.Future()
        fut.set_result(99)
        test_dict = {"a": 42, "b": fut}
        prop = await rpc.serialize_property(test_dict, [])
        self.assertDictEqual({"a": 42, "b": 99}, prop)

    @async_test
    async def test_custom_resource(self):
        res = FakeCustomResource("some-id")
        deps = []
        prop = await rpc.serialize_property(res, deps)
        self.assertListEqual([res], deps)
        self.assertEqual("some-id", prop)

    @async_test
    async def test_string_asset(self):
        asset = StringAsset("Python 3 is cool")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_asset_sig, prop[rpc._special_sig_key])
        self.assertEqual("Python 3 is cool", prop["text"])

    @async_test
    async def test_file_asset(self):
        asset = FileAsset("hello.txt")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_asset_sig, prop[rpc._special_sig_key])
        self.assertEqual("hello.txt", prop["path"])

    @async_test
    async def test_file_asset(self):
        asset = RemoteAsset("https://pulumi.io")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_asset_sig, prop[rpc._special_sig_key])
        self.assertEqual("https://pulumi.io", prop["uri"])

    @async_test
    async def test_output(self):
        res = FakeCustomResource("some-dependency")
        fut = asyncio.Future()
        fut.set_result(42)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({res}, fut, known_fut)

        deps = []
        prop = await rpc.serialize_property(out, deps)
        self.assertListEqual(deps, [res])
        self.assertEqual(42, prop)
