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
from enum import Enum
from typing import Any, Dict, List, Mapping, Optional, Sequence, cast

from google.protobuf import struct_pb2
from pulumi.resource import ComponentResource, CustomResource, ResourceOptions
from pulumi.runtime import Mocks, MockCallArgs, MockResourceArgs, ResourceModule, rpc, rpc_manager, known_types, set_mocks, settings
from pulumi import Input, Output, UNKNOWN, input_type
from pulumi.asset import (
    FileAsset,
    RemoteAsset,
    StringAsset,
    AssetArchive,
    FileArchive,
    RemoteArchive
)
import pulumi


class FakeCustomResource(CustomResource):
    def __init__(self, urn):
        self.__dict__["urn"] = Output.from_input(urn)
        self.__dict__["id"] = Output.from_input("id")


class FakeComponentResource(ComponentResource):
    def __init__(self, urn):
        self.__dict__["urn"] = Output.from_input(urn)


class MyCustomResource(CustomResource):
    def __init__(self, name: str, typ: Optional[str] = None, opts: Optional[ResourceOptions] = None):
        super(MyCustomResource, self).__init__(typ if typ is not None else "test:index:resource", name, None, opts)


class MyComponentResource(ComponentResource):
    def __init__(self, name: str, typ: Optional[str] = None, opts: Optional[ResourceOptions] = None):
        super(MyComponentResource, self).__init__(typ if typ is not None else "test:index:component", name, None, opts)


class MyResourceModule(ResourceModule):
    def version(self):
        return None

    def construct(self, name: str, typ: str, urn: str):
        if typ == "test:index:resource":
            return MyCustomResource(name, typ, ResourceOptions(urn=urn))
        elif typ == "test:index:component":
            return MyComponentResource(name, typ, ResourceOptions(urn=urn))
        else:
            raise Exception(f"unknown resource type {typ}")


class MyMocks(Mocks):
    def call(self, args: MockCallArgs):
        raise Exception(f"unknown function {args.token}")

    def new_resource(self, args: MockResourceArgs):
        if args.typ == "test:index:resource":
            return [None if settings.is_dry_run() else "id", {}]
        elif args.typ == "test:index:component":
            return [None, {}]
        else:
            raise Exception(f"unknown resource type {args.typ}")


@pulumi.output_type
class MyOutputTypeDict(dict):

    def __init__(self, values: list, items: list, keys: list):
        pulumi.set(self, "values", values)
        pulumi.set(self, "items", items)
        pulumi.set(self, "keys", keys)

    # Property with empty body.
    @property
    @pulumi.getter
    def values(self) -> str:
        """Values docstring."""
        ...

    # Property with empty body.
    @property
    @pulumi.getter
    def items(self) -> str:
        """Items docstring."""
        ...

    # Property with empty body.
    @property
    @pulumi.getter
    def keys(self) -> str:
        """Keys docstring."""
        ...


def pulumi_test(coro):
    wrapped = pulumi.runtime.test(coro)

    def wrapper(*args, **kwargs):
        settings.configure(settings.Settings())
        rpc._RESOURCE_PACKAGES.clear()
        rpc._RESOURCE_MODULES.clear()
        rpc_manager.RPC_MANAGER = rpc_manager.RPCManager()

        wrapped(*args, **kwargs)

    return wrapper


class NextSerializationTests(unittest.TestCase):
    @pulumi_test
    async def test_list(self):
        test_list = [1, 2, 3]
        props = await rpc.serialize_property(test_list, [])
        self.assertEqual(test_list, props)

    @pulumi_test
    async def test_future(self):
        fut = asyncio.Future()
        fut.set_result(42)
        prop = await rpc.serialize_property(fut, [])
        self.assertEqual(42, prop)

    @pulumi_test
    async def test_coro(self):
        async def fun():
            await asyncio.sleep(0.1)
            return 42

        prop = await rpc.serialize_property(fun(), [])
        self.assertEqual(42, prop)

    @pulumi_test
    async def test_dict(self):
        fut = asyncio.Future()
        fut.set_result(99)
        test_dict = {"a": 42, "b": fut}
        prop = await rpc.serialize_property(test_dict, [])
        self.assertDictEqual({"a": 42, "b": 99}, prop)

    @pulumi_test
    async def test_custom_resource_preview(self):
        settings.SETTINGS.dry_run = True
        rpc.register_resource_module("test", "index", MyResourceModule())
        set_mocks(MyMocks())

        res = MyCustomResource("test")
        urn = await res.urn.future()
        id = await res.id.future()

        settings.SETTINGS.feature_support["resourceReferences"] = False
        deps = []
        prop = await rpc.serialize_property(res, deps)
        self.assertListEqual([res], deps)
        self.assertEqual(id, prop)

        settings.SETTINGS.feature_support["resourceReferences"] = True
        deps = []
        prop = await rpc.serialize_property(res, deps)
        self.assertListEqual([res, res], deps)
        self.assertEqual(rpc._special_resource_sig, prop[rpc._special_sig_key])
        self.assertEqual(urn, prop["urn"])
        self.assertEqual(id, prop["id"])

        res = rpc.deserialize_properties(prop)
        self.assertTrue(isinstance(res, MyCustomResource))

        rpc._RESOURCE_MODULES.clear()
        res = rpc.deserialize_properties(prop)
        self.assertEqual(id, res)

    @pulumi_test
    async def test_custom_resource(self):
        rpc.register_resource_module("test", "index", MyResourceModule())
        set_mocks(MyMocks())

        res = MyCustomResource("test")
        urn = await res.urn.future()
        id = await res.id.future()

        settings.SETTINGS.feature_support["resourceReferences"] = False
        deps = []
        prop = await rpc.serialize_property(res, deps)
        self.assertListEqual([res], deps)
        self.assertEqual(id, prop)

        settings.SETTINGS.feature_support["resourceReferences"] = True
        deps = []
        prop = await rpc.serialize_property(res, deps)
        self.assertListEqual([res, res], deps)
        self.assertEqual(rpc._special_resource_sig, prop[rpc._special_sig_key])
        self.assertEqual(urn, prop["urn"])
        self.assertEqual(id, prop["id"])

        res = rpc.deserialize_properties(prop)
        self.assertTrue(isinstance(res, MyCustomResource))

        rpc._RESOURCE_MODULES.clear()
        res = rpc.deserialize_properties(prop)
        self.assertEqual(id, res)

    @pulumi_test
    async def test_component_resource(self):
        rpc.register_resource_module("test", "index", MyResourceModule())
        set_mocks(MyMocks())

        res = MyComponentResource("test")
        urn = await res.urn.future()

        settings.SETTINGS.feature_support["resourceReferences"] = False
        deps = []
        prop = await rpc.serialize_property(res, deps)
        self.assertListEqual([res], deps)
        self.assertEqual(urn, prop)

        settings.SETTINGS.feature_support["resourceReferences"] = True
        deps = []
        prop = await rpc.serialize_property(res, deps)
        self.assertListEqual([res], deps)
        self.assertEqual(rpc._special_resource_sig, prop[rpc._special_sig_key])
        self.assertEqual(urn, prop["urn"])

        res = rpc.deserialize_properties(prop)
        self.assertTrue(isinstance(res, MyComponentResource))

        rpc._RESOURCE_MODULES.clear()
        res = rpc.deserialize_properties(prop)
        self.assertEqual(urn, res)

    @pulumi_test
    async def test_string_asset(self):
        asset = StringAsset("Python 3 is cool")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_asset_sig, prop[rpc._special_sig_key])
        self.assertEqual("Python 3 is cool", prop["text"])

    @pulumi_test
    async def test_file_asset(self):
        asset = FileAsset("hello.txt")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_asset_sig, prop[rpc._special_sig_key])
        self.assertEqual("hello.txt", prop["path"])

    @pulumi_test
    async def test_remote_asset(self):
        asset = RemoteAsset("https://pulumi.com")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_asset_sig, prop[rpc._special_sig_key])
        self.assertEqual("https://pulumi.com", prop["uri"])

    @pulumi_test
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

        known_fut = asyncio.Future()
        known_fut.set_result(False)
        out = Output(set(), fut, known_fut)

        # For compatibility, future() should still return 42 even if the value is unknown.
        prop = await out.future()
        self.assertEqual(42, prop)

        fut = asyncio.Future()
        fut.set_result(UNKNOWN)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output(set(), fut, known_fut)

        # For compatibility, is_known() should return False and future() should return None when the value contains
        # first-class unknowns.
        self.assertEqual(False, await out.is_known())
        self.assertEqual(None, await out.future())

        # If the caller of future() explicitly accepts first-class unknowns, they should be present in the result.
        self.assertEqual(UNKNOWN, await out.future(with_unknowns=True))

    @pulumi_test
    async def test_output_all(self):
        res = FakeCustomResource("some-resource")
        fut = asyncio.Future()
        fut.set_result(42)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({res}, fut, known_fut)

        other = Output.from_input(99)
        combined = Output.all(out, other)
        combined_dict = Output.all(out=out, other=other)
        deps = []
        prop = await rpc.serialize_property(combined, deps)
        prop_dict = await rpc.serialize_property(combined_dict, deps)
        self.assertSetEqual(set(deps), {res})
        self.assertEqual([42, 99], prop)
        self.assertEqual({"out": 42, "other": 99}, prop_dict)

    @pulumi_test
    async def test_output_all_no_inputs(self):
        empty_all = Output.all()
        deps = []
        prop = await rpc.serialize_property(empty_all, deps)
        self.assertEqual([], prop)

    @pulumi_test
    async def test_output_all_failure_mixed_inputs(self):
        res = FakeCustomResource("some-resource")
        fut = asyncio.Future()
        fut.set_result(42)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({res}, fut, known_fut)

        other = Output.from_input(99)
        self.assertRaises(ValueError, Output.all, out, other=other)
        self.assertRaisesRegex(ValueError, "Output.all() was supplied a mix of named and unnamed inputs")

    @pulumi_test
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
        combined_dict = Output.all(out=out, other_out=other_out)
        deps = []
        prop = await rpc.serialize_property(combined, deps)
        prop_dict = await rpc.serialize_property(combined_dict, deps)
        self.assertSetEqual(set(deps), {res, other})
        self.assertEqual([42, 99], prop)
        self.assertEqual({"out": 42, "other_out": 99}, prop_dict)

    @pulumi_test
    async def test_output_all_known_if_all_are_known(self):
        res = FakeCustomResource("some-resource")
        fut = asyncio.Future()
        fut.set_result(42)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output({res}, fut, known_fut)

        other = FakeCustomResource("some-other-resource")
        other_fut = asyncio.Future()
        other_fut.set_result(UNKNOWN)  # <- not known
        other_known_fut = asyncio.Future()
        other_known_fut.set_result(False)
        other_out = Output({other}, other_fut, other_known_fut)

        combined = Output.all(out, other_out)
        combined_dict = Output.all(out=out, other_out=other_out)
        deps = []
        prop = await rpc.serialize_property(combined, deps)
        prop_dict = await rpc.serialize_property(combined_dict, deps)
        self.assertSetEqual(set(deps), {res, other})

        # The contents of the list are unknown if any of the Outputs used to
        # create it were unknown.
        self.assertEqual(rpc.UNKNOWN, prop)
        self.assertEqual(rpc.UNKNOWN, prop_dict)

    @pulumi_test
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

    @pulumi_test
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

    @pulumi_test
    async def test_remote_archive(self):
        asset = RemoteArchive("https://pulumi.com")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_archive_sig, prop[rpc._special_sig_key])
        self.assertEqual("https://pulumi.com", prop["uri"])

    @pulumi_test
    async def test_file_archive(self):
        asset = FileArchive("foo.tar.gz")
        prop = await rpc.serialize_property(asset, [])
        self.assertEqual(rpc._special_archive_sig, prop[rpc._special_sig_key])
        self.assertEqual("foo.tar.gz", prop["path"])

    @pulumi_test
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

    @pulumi_test
    async def test_string(self):
        # Ensure strings are serialized as strings (and not sequences).
        prop = await rpc.serialize_property("hello world", [])
        self.assertEqual("hello world", prop)

    @pulumi_test
    async def test_unsupported_sequences(self):
        cases = [
            ("hi", 42),
            range(10),
            memoryview(bytes(10)),
            bytes(10),
            bytearray(10),
        ]

        for case in cases:
            with self.assertRaises(ValueError):
                await rpc.serialize_property(case, [])

    @pulumi_test
    async def test_distinguished_unknown_output(self):
        fut = asyncio.Future()
        fut.set_result(UNKNOWN)
        known_fut = asyncio.Future()
        known_fut.set_result(True)
        out = Output(set(), fut, known_fut)
        self.assertFalse(await out.is_known())

        fut = asyncio.Future()
        fut.set_result(["foo", UNKNOWN])
        out = Output(set(), fut, known_fut)
        self.assertFalse(await out.is_known())

        fut = asyncio.Future()
        fut.set_result({"foo": "foo", "bar": UNKNOWN})
        out = Output(set(), fut, known_fut)
        self.assertFalse(await out.is_known())

    def create_output(self, val: Any, is_known: bool, is_secret: Optional[bool] = None):
        fut = asyncio.Future()
        fut.set_result(val)
        known_fut = asyncio.Future()
        known_fut.set_result(is_known)
        if is_secret is not None:
            is_secret_fut = asyncio.Future()
            is_secret_fut.set_result(True)
            return Output(set(), fut, known_fut, is_secret_fut)
        return Output(set(), fut, known_fut)

    @pulumi_test
    async def test_apply_can_run_on_known_value_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: v + 1)

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), 1)

    @pulumi_test
    async def test_apply_can_run_on_known_awaitable_value_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True)

        def apply(v):
            fut = asyncio.Future()
            fut.set_result("inner")
            return fut
        r = out.apply(apply)

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_can_run_on_known_known_output_value_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=True))

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_can_run_on_known_unknown_output_value_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=False))

        self.assertFalse(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_produces_unknown_default_on_unknown_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: v + 1)

        self.assertFalse(await r.is_known())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_produces_unknown_default_on_unknown_awaitable_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False)

        def apply(v):
            fut = asyncio.Future()
            fut.set_result("inner")
            return fut
        r = out.apply(apply)

        self.assertFalse(await r.is_known())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_produces_unknown_default_on_unknown_known_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: self.create_output("inner", is_known=True))

        self.assertFalse(await r.is_known())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_produces_unknown_default_on_unknown_unknown_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: self.create_output("inner", is_known=False))

        self.assertFalse(await r.is_known())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_preserves_secret_on_known_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True, is_secret=True)
        r = out.apply(lambda v: v + 1)

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), 1)

    @pulumi_test
    async def test_apply_preserves_secret_on_known_awaitable_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True, is_secret=True)

        def apply(v):
            fut = asyncio.Future()
            fut.set_result("inner")
            return fut
        r = out.apply(apply)

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_known_known_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True, is_secret=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=True))

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_known_unknown_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True, is_secret=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=False))

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_unknown_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False, is_secret=True)
        r = out.apply(lambda v: v + 1)

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_preserves_secret_on_unknown_awaitable_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False, is_secret=True)

        def apply(v):
            fut = asyncio.Future()
            fut.set_result("inner")
            return fut
        r = out.apply(apply)

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_preserves_secret_on_unknown_known_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False, is_secret=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=True))

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_preserves_secret_on_unknown_unknown_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False, is_secret=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=False))

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_propagates_secret_on_known_known_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=True, is_secret=True))

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_propagates_secret_on_known_unknown_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=False, is_secret=True))

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_does_not_propagate_secret_on_unknown_known_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: self.create_output("inner", is_known=True, is_secret=True))

        self.assertFalse(await r.is_known())
        self.assertFalse(await r.is_secret())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_does_not_propagate_secret_on_unknown_unknown_output_during_preview(self):
        settings.SETTINGS.dry_run = True

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: self.create_output("inner", is_known=False, is_secret=True))

        self.assertFalse(await r.is_known())
        self.assertFalse(await r.is_secret())
        self.assertEqual(await r.future(), None)

    @pulumi_test
    async def test_apply_can_run_on_known_value(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: v + 1)

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), 1)

    @pulumi_test
    async def test_apply_can_run_on_known_awaitable_value(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True)

        def apply(v):
            fut = asyncio.Future()
            fut.set_result("inner")
            return fut
        r = out.apply(apply)

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_can_run_on_known_known_output_value(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=True))

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_can_run_on_known_unknown_output_value(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=False))

        self.assertFalse(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_produces_known_on_unknown(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: v + 1)

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), 1)

    @pulumi_test
    async def test_apply_produces_known_on_unknown_awaitable(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False)

        def apply(v):
            fut = asyncio.Future()
            fut.set_result("inner")
            return fut
        r = out.apply(apply)

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_produces_known_on_unknown_known_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: self.create_output("inner", is_known=True))

        self.assertTrue(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_produces_unknown_on_unknown_unknown_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: self.create_output("inner", is_known=False))

        self.assertFalse(await r.is_known())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_known(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True, is_secret=True)
        r = out.apply(lambda v: v + 1)

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), 1)

    @pulumi_test
    async def test_apply_preserves_secret_on_known_awaitable(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True, is_secret=True)

        def apply(v):
            fut = asyncio.Future()
            fut.set_result("inner")
            return fut
        r = out.apply(apply)

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_known_known_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True, is_secret=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=True))

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_known_unknown_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True, is_secret=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=False))

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_unknown(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False, is_secret=True)
        r = out.apply(lambda v: v + 1)

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), 1)

    @pulumi_test
    async def test_apply_preserves_secret_on_unknown_awaitable(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False, is_secret=True)

        def apply(v):
            fut = asyncio.Future()
            fut.set_result("inner")
            return fut
        r = out.apply(apply)

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_unknown_known_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False, is_secret=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=True))

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_preserves_secret_on_unknown_unknown_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False, is_secret=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=False))

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_propagates_secret_on_known_known_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=True, is_secret=True))

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_propagates_secret_on_known_unknown_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=True)
        r = out.apply(lambda v: self.create_output("inner", is_known=False, is_secret=True))

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_propagates_secret_on_unknown_known_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: self.create_output("inner", is_known=True, is_secret=True))

        self.assertTrue(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_apply_propagates_secret_on_unknown_unknown_output(self):
        settings.SETTINGS.dry_run = False

        out = self.create_output(0, is_known=False)
        r = out.apply(lambda v: self.create_output("inner", is_known=False, is_secret=True))

        self.assertFalse(await r.is_known())
        self.assertTrue(await r.is_secret())
        self.assertEqual(await r.future(), "inner")

    @pulumi_test
    async def test_dangerous_prop_output(self):
        out = self.create_output(MyOutputTypeDict(values=["foo", "bar"],
                                                  items=["yellow", "purple"],
                                                  keys=["yes", "no"]), is_known=True)
        prop = await rpc.serialize_property(out, [])

        self.assertTrue(await out.is_known())
        self.assertEqual(prop["values"], ["foo", "bar"])
        self.assertEqual(prop["items"], ["yellow", "purple"])
        self.assertEqual(prop["keys"], ["yes", "no"])

    @pulumi_test
    async def test_apply_unknown_output(self):
        out = self.create_output("foo", is_known=True)

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

    @pulumi_test
    async def test_lifted_unknown(self):
        settings.SETTINGS.dry_run = True

        fut = asyncio.Future()
        fut.set_result(UNKNOWN)
        out = Output.from_input({"foo": "foo", "bar": UNKNOWN, "baz": fut})

        self.assertFalse(await out.is_known())

        r1 = out["foo"]
        self.assertTrue(await r1.is_known())
        self.assertEqual(await r1.future(with_unknowns=True), "foo")

        r2 = out["bar"]
        self.assertFalse(await r2.is_known())
        self.assertEqual(await r2.future(with_unknowns=True), UNKNOWN)

        r3 = out["baz"]
        self.assertFalse(await r3.is_known())
        self.assertEqual(await r3.future(with_unknowns=True), UNKNOWN)

        r4 = out["baz"]["qux"]
        self.assertFalse(await r4.is_known())
        self.assertEqual(await r4.future(with_unknowns=True), UNKNOWN)

        out = Output.from_input(["foo", UNKNOWN])

        r5 = out[0]
        self.assertTrue(await r5.is_known())
        self.assertEqual(await r5.future(with_unknowns=True), "foo")

        r6 = out[1]
        self.assertFalse(await r6.is_known())
        self.assertEqual(await r6.future(with_unknowns=True), UNKNOWN)

        out = Output.all(Output.from_input("foo"), Output.from_input(UNKNOWN),
                         Output.from_input([Output.from_input(UNKNOWN), Output.from_input("bar")]))

        self.assertFalse(await out.is_known())

        r7 = out[0]
        self.assertTrue(await r7.is_known())
        self.assertEqual(await r7.future(with_unknowns=True), "foo")

        r8 = out[1]
        self.assertFalse(await r8.is_known())
        self.assertEqual(await r8.future(with_unknowns=True), UNKNOWN)

        r9 = out[2]
        self.assertFalse(await r9.is_known())

        r10 = r9[0]
        self.assertFalse(await r10.is_known())
        self.assertEqual(await r10.future(with_unknowns=True), UNKNOWN)

        r11 = r9[1]
        self.assertTrue(await r11.is_known())
        self.assertEqual(await r11.future(with_unknowns=True), "bar")

        out_dict = Output.all(foo=Output.from_input("foo"), unknown=Output.from_input(UNKNOWN),
                              arr=Output.from_input([Output.from_input(UNKNOWN), Output.from_input("bar")]))

        self.assertFalse(await out_dict.is_known())

        r12 = out_dict["foo"]
        self.assertTrue(await r12.is_known())
        self.assertEqual(await r12.future(with_unknowns=True), "foo")

        r13 = out_dict["unknown"]
        self.assertFalse(await r13.is_known())
        self.assertEqual(await r13.future(with_unknowns=True), UNKNOWN)

        r14 = out_dict["arr"]
        self.assertFalse(await r14.is_known())

        r15 = r14[0]
        self.assertFalse(await r15.is_known())
        self.assertEqual(await r15.future(with_unknowns=True), UNKNOWN)

        r16 = r14[1]
        self.assertTrue(await r16.is_known())
        self.assertEqual(await r16.future(with_unknowns=True), "bar")


    @pulumi_test
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

        out = Output(set(), value(), is_known())

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
        secret_value = {rpc._special_sig_key: rpc._special_secret_sig, "value": "a secret value"}
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

    def test_internal_property(self):
        all_props = struct_pb2.Struct()
        all_props["a"] = "b"
        all_props["__defaults"] = []
        all_props["c"] = {"foo": "bar", "__defaults": []}
        all_props["__provider"] = "serialized_dynamic_provider"
        all_props["__other"] = "baz"

        val = rpc.deserialize_properties(all_props)
        self.assertEqual({
            "a": "b",
            "c": {"foo": "bar"},
            "__provider": "serialized_dynamic_provider",
        }, val)


@input_type
class FooArgs:
    first_arg: Input[str] = pulumi.property("firstArg")
    second_arg: Optional[Input[float]] = pulumi.property("secondArg")

    def __init__(self, first_arg: Input[str], second_arg: Optional[Input[float]] = None):
        pulumi.set(self, "first_arg", first_arg)
        pulumi.set(self, "second_arg", second_arg)


@input_type
class ListDictInputArgs:
    a: List[Input[str]]
    b: Sequence[Input[str]]
    c: Dict[str, Input[str]]
    d: Mapping[str, Input[str]]

    def __init__(self,
                 a: List[Input[str]],
                 b: Sequence[Input[str]],
                 c: Dict[str, Input[str]],
                 d: Mapping[str, Input[str]]):
        pulumi.set(self, "a", a)
        pulumi.set(self, "b", b)
        pulumi.set(self, "c", c)
        pulumi.set(self, "d", d)


@input_type
class BarArgs:
    tag_args: Input[dict] = pulumi.property("tagArgs")

    def __init__(self, tag_args: Input[dict]):
        pulumi.set(self, "tag_args", tag_args)


class InputTypeSerializationTests(unittest.TestCase):
    @pulumi_test
    async def test_simple_input_type(self):
        it = FooArgs(first_arg="hello", second_arg=42)
        prop = await rpc.serialize_property(it, [])
        self.assertEqual({"firstArg": "hello", "secondArg": 42}, prop)

    @pulumi_test
    async def test_list_dict_input_type(self):
        it = ListDictInputArgs(a=["hi"], b=["there"], c={"hello": "world"}, d={"foo": "bar"})
        prop = await rpc.serialize_property(it, [])
        self.assertEqual({
            "a": ["hi"],
            "b": ["there"],
            "c": {"hello": "world"},
            "d": {"foo": "bar"}
        }, prop)

    @pulumi_test
    async def test_input_type_with_dict_property(self):
        def transformer(prop: str) -> str:
            return {
                "tag_args": "a",
                "tagArgs": "b",
                "foo_bar": "c",
            }.get(prop) or prop

        it = BarArgs({"foo_bar": "hello", "foo_baz": "world"})
        prop = await rpc.serialize_property(it, [], transformer)
        # Input type keys are not transformed, but keys of nested
        # dicts are still transformed.
        self.assertEqual({
            "tagArgs": {
                "c": "hello",
                "foo_baz": "world",
            },
        }, prop)


class StrEnum(str, Enum):
    ONE = "one"
    ZERO = "zero"


class IntEnum(int, Enum):
    ONE = 1
    ZERO = 0


class FloatEnum(float, Enum):
    ONE = 1.0
    ZERO_POINT_ONE = 0.1


class EnumSerializationTests(unittest.TestCase):
    @pulumi_test
    async def test_string_enum(self):
        one = StrEnum.ONE
        prop = await rpc.serialize_property(one, [])
        self.assertEqual(StrEnum.ONE, prop)

    @pulumi_test
    async def test_int_enum(self):
        one = IntEnum.ONE
        prop = await rpc.serialize_property(one, [])
        self.assertEqual(IntEnum.ONE, prop)

    @pulumi_test
    async def test_float_enum(self):
        one = FloatEnum.ZERO_POINT_ONE
        prop = await rpc.serialize_property(one, [])
        self.assertEqual(FloatEnum.ZERO_POINT_ONE, prop)


@pulumi.input_type
class SomeFooArgs:
    def __init__(self, the_first: str, the_second: Mapping[str, str]):
        pulumi.set(self, "the_first", the_first)
        pulumi.set(self, "the_second", the_second)

    @property
    @pulumi.getter(name="theFirst")
    def the_first(self) -> str:
        ...

    @property
    @pulumi.getter(name="theSecond")
    def the_second(self) -> Mapping[str, str]:
        ...


@pulumi.input_type
class SerializationArgs:
    def __init__(self,
                 some_value: pulumi.Input[str],
                 some_foo: pulumi.Input[pulumi.InputType[SomeFooArgs]],
                 some_bar: pulumi.Input[Mapping[str, pulumi.Input[pulumi.InputType[SomeFooArgs]]]]):
        pulumi.set(self, "some_value", some_value)
        pulumi.set(self, "some_foo", some_foo)
        pulumi.set(self, "some_bar", some_bar)

    @property
    @pulumi.getter(name="someValue")
    def some_value(self) -> pulumi.Input[str]:
        ...

    @property
    @pulumi.getter(name="someFoo")
    def some_foo(self) -> pulumi.Input[pulumi.InputType[SomeFooArgs]]:
        ...

    @property
    @pulumi.getter(name="someBar")
    def some_bar(self) -> pulumi.Input[Mapping[str, pulumi.Input[pulumi.InputType[SomeFooArgs]]]]:
        ...


@pulumi.output_type
class SomeFooOutput(dict):
    def __init__(self, the_first: str, the_second: Mapping[str, str]):
        pulumi.set(self, "the_first", the_first)
        pulumi.set(self, "the_second", the_second)

    @property
    @pulumi.getter(name="theFirst")
    def the_first(self) -> str:
        ...

    @property
    @pulumi.getter(name="theSecond")
    def the_second(self) -> Mapping[str, str]:
        ...


@pulumi.output_type
class DeserializationOutput(dict):
    def __init__(self,
                 some_value: str,
                 some_foo: SomeFooOutput,
                 some_bar: Mapping[str, SomeFooOutput]):
        pulumi.set(self, "some_value", some_value)
        pulumi.set(self, "some_foo", some_foo)
        pulumi.set(self, "some_bar", some_bar)

    @property
    @pulumi.getter(name="someValue")
    def some_value(self) -> str:
        ...

    @property
    @pulumi.getter(name="someFoo")
    def some_foo(self) -> SomeFooOutput:
        ...

    @property
    @pulumi.getter(name="someBar")
    def some_bar(self) -> Mapping[str, SomeFooOutput]:
        ...


class TypeMetaDataSerializationTests(unittest.TestCase):
    @pulumi_test
    async def test_serialize(self):
        # The transformer should never be called.
        def transformer(key: str) -> str:
            raise Exception(f"Should not be raised for key '{key}'")

        tests = [
            {
                "some_value": "hello",
                "some_foo": SomeFooArgs("first", {"the_first": "there"}),
                "some_bar": {"a": SomeFooArgs("second", {"the_second": "later"})},
            },
            {
                "some_value": "hello",
                "some_foo": {"the_first": "first", "the_second": {"the_first": "there"}},
                "some_bar": {"a": {"the_first": "second", "the_second": {"the_second": "later"}}},
            },
            {
                "some_value": "hello",
                "some_foo": {"theFirst": "first", "theSecond": {"the_first": "there"}},
                "some_bar": {"a": {"theFirst": "second", "theSecond": {"the_second": "later"}}},
            },
        ]

        for props in tests:
            result = await rpc.serialize_properties(props, {}, transformer, SerializationArgs)

            self.assertEqual("hello", result["someValue"])

            self.assertIsInstance(result["someFoo"], struct_pb2.Struct)
            self.assertEqual("first", result["someFoo"]["theFirst"])
            self.assertIsInstance(result["someFoo"]["theSecond"], struct_pb2.Struct)
            self.assertEqual("there", result["someFoo"]["theSecond"]["the_first"])

            self.assertIsInstance(result["someBar"], struct_pb2.Struct)
            self.assertIsInstance(result["someBar"]["a"], struct_pb2.Struct)
            self.assertEqual("second", result["someBar"]["a"]["theFirst"])
            self.assertIsInstance(result["someBar"]["a"]["theSecond"], struct_pb2.Struct)
            self.assertEqual("later", result["someBar"]["a"]["theSecond"]["the_second"])

    @pulumi_test
    async def test_output_translation(self):
        # The transformer should never be called.
        def transformer(key: str) -> str:
            raise Exception(f"Should not be raised for key '{key}'")

        output = {
            "someValue": "hello",
            "someFoo": {"theFirst": "first", "theSecond": {"the_first": "there"}},
            "someBar": {"a": {"theFirst": "second", "theSecond": {"the_second": "later"}}},
        }

        translated = rpc.translate_output_properties(output, transformer, DeserializationOutput, True)

        self.assertIsInstance(translated, DeserializationOutput)
        result = cast(DeserializationOutput, translated)

        self.assertEqual("hello", result.some_value)

        self.assertIsInstance(result.some_foo, SomeFooOutput)
        self.assertIsInstance(result.some_foo, dict)
        self.assertEqual("first", result.some_foo.the_first)
        self.assertEqual("first", result.some_foo["the_first"])
        self.assertEqual({"the_first": "there"}, result.some_foo.the_second)
        self.assertEqual({"the_first": "there"}, result.some_foo["the_second"])

        self.assertIsInstance(result.some_bar, dict)
        self.assertEqual(1, len(result.some_bar))
        self.assertIsInstance(result.some_bar["a"], SomeFooOutput)
        self.assertIsInstance(result.some_bar["a"], dict)
        self.assertEqual("second", result.some_bar["a"].the_first)
        self.assertEqual("second", result.some_bar["a"]["the_first"])
        self.assertEqual({"the_second": "later"}, result.some_bar["a"].the_second)
        self.assertEqual({"the_second": "later"}, result.some_bar["a"]["the_second"])
