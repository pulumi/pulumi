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

from google.protobuf import struct_pb2
from pulumi.resource import CustomResource
from pulumi.runtime import rpc, known_types
from pulumi.output import Output, UNKNOWN
from pulumi.asset import (
    FileAsset,
    RemoteAsset,
    StringAsset,
    AssetArchive,
    FileArchive,
    RemoteArchive
)


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
        loop.run_until_complete(coro(*args, **kwargs))
        loop.close()
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
    async def test_remote_asset(self):
        asset = RemoteAsset("https://pulumi.com")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_asset_sig, prop[rpc._special_sig_key])
        self.assertEqual("https://pulumi.com", prop["uri"])

    @async_test
    async def test_output(self):
        existing = FakeCustomResource("existing-dependency")
        res = FakeCustomResource("some-dependency")
        fut = asyncio.Future()
        fut.set_result(42)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({res}, fut, known_fut)

        deps = [existing]
        prop = await rpc.serialize_property(out, deps)
        self.assertListEqual(deps, [existing, res])
        self.assertEqual(42, prop)

    @async_test
    async def test_output_all(self):
        res = FakeCustomResource("some-resource")
        fut = asyncio.Future()
        fut.set_result(42)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({res}, fut, known_fut)

        other = Output.from_input(99)
        combined = Output.all(out, other)
        deps = []
        prop = await rpc.serialize_property(combined, deps)
        self.assertListEqual(deps, [res])
        self.assertEqual([42, 99], prop)

    @async_test
    async def test_output_all_composes_dependencies(self):
        res = FakeCustomResource("some-resource")
        fut = asyncio.Future()
        fut.set_result(42)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({res}, fut, known_fut)

        other = FakeCustomResource("some-other-resource")
        other_fut = asyncio.Future()
        other_fut.set_result(99)
        other_known_fut = asyncio.Future()
        other_known_fut.set_result(True)
        other_out = Output({other}, other_fut, other_known_fut)

        combined = Output.all(out, other_out)
        deps = []
        prop = await rpc.serialize_property(combined, deps)
        self.assertSetEqual(set(deps), {res, other})
        self.assertEqual([42, 99], prop)

    @async_test
    async def test_output_all_known_if_all_are_known(self):
        res = FakeCustomResource("some-resource")
        fut = asyncio.Future()
        fut.set_result(42)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({res}, fut, known_fut)

        other = FakeCustomResource("some-other-resource")
        other_fut = asyncio.Future()
        other_fut.set_result(UNKNOWN) # <- not known
        other_known_fut = asyncio.Future()
        other_known_fut.set_result(False)
        other_out = Output({other}, other_fut, other_known_fut)

        combined = Output.all(out, other_out)
        deps = []
        prop = await rpc.serialize_property(combined, deps)
        self.assertSetEqual(set(deps), {res, other})

        # The contents of the list are unknown if any of the Outputs used to
        # create it were unknown.
        self.assertEqual(rpc.UNKNOWN, prop)


    @async_test
    async def test_unknown_output(self):
        res = FakeCustomResource("some-dependency")
        fut = asyncio.Future()
        fut.set_result(None)
        known_fut = asyncio.Future()
        known_fut.set_result(False)
        out = Output({res}, fut, known_fut)
        deps = []
        prop = await rpc.serialize_property(out, deps)
        self.assertListEqual(deps, [res])
        self.assertEqual(rpc.UNKNOWN, prop)

    @async_test
    async def test_asset_archive(self):
        archive = AssetArchive({
            "foo": StringAsset("bar")
        })

        deps = []
        prop = await rpc.serialize_property(archive, deps)
        self.assertDictEqual({
            rpc._special_sig_key: rpc._special_archive_sig,
            "assets": {
                "foo": {
                    rpc._special_sig_key: rpc._special_asset_sig,
                    "text": "bar"
                }
            }
        }, prop)

    @async_test
    async def test_remote_archive(self):
        asset = RemoteArchive("https://pulumi.com")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_archive_sig, prop[rpc._special_sig_key])
        self.assertEqual("https://pulumi.com", prop["uri"])

    @async_test
    async def test_file_archive(self):
        asset = FileArchive("foo.tar.gz")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_archive_sig, prop[rpc._special_sig_key])
        self.assertEqual("foo.tar.gz", prop["path"])

    @async_test
    async def test_bad_inputs(self):
        class MyClass:
            def __init__(self):
                self.prop = "oh no!"

        error = None
        try:
            prop = await rpc.serialize_property(MyClass(), [])
        except ValueError as err:
            error = err

        self.assertIsNotNone(error)
        self.assertEqual("unexpected input of type MyClass", str(error))

    @async_test
    async def test_distinguished_unknown_output(self):
        fut = asyncio.Future()
        fut.set_result(UNKNOWN)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({}, fut, known_fut)
        self.assertFalse(await out.is_known())

        fut = asyncio.Future()
        fut.set_result(["foo", UNKNOWN])
        out = Output({}, fut, known_fut)
        self.assertFalse(await out.is_known())

        fut = asyncio.Future()
        fut.set_result({"foo": "foo", "bar": UNKNOWN})
        out = Output({}, fut, known_fut)
        self.assertFalse(await out.is_known())

    @async_test
    async def test_apply_unknown_output(self):
        fut = asyncio.Future()
        fut.set_result("foo")
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({}, fut, known_fut)

        r1 = out.apply(lambda v: UNKNOWN)
        r2 = out.apply(lambda v: [v, UNKNOWN])
        r3 = out.apply(lambda v: {"v": v, "unknown": UNKNOWN})
        r4 = out.apply(lambda v: UNKNOWN).apply(lambda v: v, True)
        r5 = out.apply(lambda v: [v, UNKNOWN]).apply(lambda v: v, True)
        r6 = out.apply(lambda v: {"v": v, "unknown": UNKNOWN}).apply(lambda v: v, True)

        self.assertFalse(await r1.is_known())
        self.assertFalse(await r2.is_known())
        self.assertFalse(await r3.is_known())
        self.assertFalse(await r4.is_known())
        self.assertFalse(await r5.is_known())
        self.assertFalse(await r6.is_known())

    @async_test
    async def test_lifted_unknown(self):
        fut = asyncio.Future()
        fut.set_result(UNKNOWN)
        out = Output.from_input({ "foo": "foo", "bar": UNKNOWN, "baz": fut})

        self.assertFalse(await out.is_known())

        r1 = out["foo"]
        self.assertTrue(await r1.is_known())
        self.assertEqual(await r1.future(), "foo")

        r2 = out["bar"]
        self.assertFalse(await r2.is_known())
        self.assertEqual(await r2.future(), UNKNOWN)

        r3 = out["baz"]
        self.assertFalse(await r3.is_known())
        self.assertEqual(await r3.future(), UNKNOWN)

    @async_test
    async def test_output_coros(self):
        # Ensure that Outputs function properly when the input value and is_known are coroutines. If the implementation
        # is not careful to wrap these coroutines in Futures, they will be awaited more than once and the runtime will
        # throw.
        async def value():
            await asyncio.sleep(0)
            return 42
        async def is_known():
            await asyncio.sleep(0)
            return True

        out = Output({}, value(), is_known())

        self.assertTrue(await out.is_known())
        self.assertEqual(42, await out.future())
        self.assertEqual(42, await out.apply(lambda v: v).future())


class DeserializationTests(unittest.TestCase):
    def test_unsupported_sig(self):
        struct = struct_pb2.Struct()
        struct[rpc._special_sig_key] = "foobar"

        error = None
        try:
            rpc.deserialize_property(struct)
        except  AssertionError as err:
            error = err
        self.assertIsNotNone(error)

    def test_secret_push_up(self):
        secret_value = {rpc._special_sig_key: rpc._special_secret_sig, "value": "a secret value" }
        all_props = struct_pb2.Struct()
        all_props["regular"] = "a normal value"
        all_props["list"] = ["a normal value", "another value", secret_value]
        all_props["map"] = {"regular": "a normal value", "secret": secret_value}
        all_props["mapWithList"] = {"regular": "a normal value", "list": ["a normal value", secret_value]}
        all_props["listWithMap"] = [{"regular": "a normal value", "secret": secret_value}]


        val = rpc.deserialize_properties(all_props)
        self.assertEqual(all_props["regular"], val["regular"])

        self.assertIsInstance(val["list"], dict)
        self.assertEqual(val["list"][rpc._special_sig_key], rpc._special_secret_sig)
        self.assertEqual(val["list"]["value"][0], "a normal value")
        self.assertEqual(val["list"]["value"][1], "another value")
        self.assertEqual(val["list"]["value"][2], "a secret value")

        self.assertIsInstance(val["map"], dict)
        self.assertEqual(val["map"][rpc._special_sig_key], rpc._special_secret_sig)
        self.assertEqual(val["map"]["value"]["regular"], "a normal value")
        self.assertEqual(val["map"]["value"]["secret"], "a secret value")

        self.assertIsInstance(val["mapWithList"], dict)
        self.assertEqual(val["mapWithList"][rpc._special_sig_key], rpc._special_secret_sig)
        self.assertEqual(val["mapWithList"]["value"]["regular"], "a normal value")
        self.assertEqual(val["mapWithList"]["value"]["list"][0], "a normal value")
        self.assertEqual(val["mapWithList"]["value"]["list"][1], "a secret value")

        self.assertIsInstance(val["listWithMap"], dict)
        self.assertEqual(val["listWithMap"][rpc._special_sig_key], rpc._special_secret_sig)
        self.assertEqual(val["listWithMap"]["value"][0]["regular"], "a normal value")
        self.assertEqual(val["listWithMap"]["value"][0]["secret"], "a secret value")
