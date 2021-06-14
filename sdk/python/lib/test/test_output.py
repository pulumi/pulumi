# Copyright 2016-2021, Pulumi Corporation.
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

import unittest
from typing import Mapping, Optional, Sequence, cast

from pulumi.runtime import rpc, rpc_manager, settings
from pulumi import Output
import pulumi


def pulumi_test(coro):
    wrapped = pulumi.runtime.test(coro)
    def wrapper(*args, **kwargs):
        settings.configure(settings.Settings())
        rpc._RESOURCE_PACKAGES.clear()
        rpc._RESOURCE_MODULES.clear()
        rpc_manager.RPC_MANAGER = rpc_manager.RPCManager()

        wrapped(*args, **kwargs)

    return wrapper


class OutputSecretTests(unittest.TestCase):
    @pulumi_test
    async def test_secret(self):
        x = Output.secret("foo")
        is_secret = await x.is_secret()
        self.assertTrue(is_secret)

    @pulumi_test
    async def test_unsecret(self):
        x = Output.secret("foo")
        x_is_secret = await x.is_secret()
        self.assertTrue(x_is_secret)

        y = Output.unsecret(x)
        y_val = await y.future()
        y_is_secret = await y.is_secret()
        self.assertEqual(y_val, "foo")
        self.assertFalse(y_is_secret)


class OutputFromInputTests(unittest.TestCase):
    @pulumi_test
    async def test_unwrap_empty_dict(self):
        x = Output.from_input({})
        x_val = await x.future()
        self.assertEqual(x_val, {})

    @pulumi_test
    async def test_unwrap_dict(self):
        x = Output.from_input({"hello": Output.from_input("world")})
        x_val = await x.future()
        self.assertEqual(x_val, {"hello": "world"})

    @pulumi_test
    async def test_unwrap_dict_secret(self):
        x = Output.from_input({"hello": Output.secret("world")})
        x_val = await x.future()
        self.assertEqual(x_val, {"hello": "world"})

    @pulumi_test
    async def test_unwrap_dict_dict(self):
        x = Output.from_input({"hello": {"foo": Output.from_input("bar")}})
        x_val = await x.future()
        self.assertEqual(x_val, {"hello": {"foo": "bar"}})

    @pulumi_test
    async def test_unwrap_dict_list(self):
        x = Output.from_input({"hello": ["foo", Output.from_input("bar")]})
        x_val = await x.future()
        self.assertEqual(x_val, {"hello": ["foo", "bar"]})

    @pulumi_test
    async def test_unwrap_empty_list(self):
        x = Output.from_input([])
        x_val = await x.future()
        self.assertEqual(x_val, [])

    @pulumi_test
    async def test_unwrap_list(self):
        x = Output.from_input(["hello", Output.from_input("world")])
        x_val = await x.future()
        self.assertEqual(x_val, ["hello", "world"])

    @pulumi_test
    async def test_unwrap_list_list(self):
        x = Output.from_input(["hello", ["foo", Output.from_input("bar")]])
        x_val = await x.future()
        self.assertEqual(x_val, ["hello", ["foo", "bar"]])

    @pulumi_test
    async def test_unwrap_list_dict(self):
        x = Output.from_input(["hello", {"foo": Output.from_input("bar")}])
        x_val = await x.future()
        self.assertEqual(x_val, ["hello", {"foo": "bar"}])

    @pulumi_test
    async def test_deeply_nested_objects(self):
        o1 = {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": Output.from_input("a")}}}}}}}}}}}
        o2 = {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": "a"}}}}}}}}}}}
        x = Output.from_input(o1)
        x_val = await x.future()
        self.assertEqual(x_val, o2)

    @pulumi.input_type
    class FooArgs:
        def __init__(self, *,
                     foo: Optional[pulumi.Input[str]] = None,
                     bar: Optional[pulumi.Input[Sequence[pulumi.Input[str]]]] = None,
                     baz: Optional[pulumi.Input[Mapping[str, pulumi.Input[str]]]] = None,
                     nested: Optional[pulumi.Input[pulumi.InputType['NestedArgs']]] = None):
            if foo is not None:
                pulumi.set(self, "foo", foo)
            if bar is not None:
                pulumi.set(self, "bar", bar)
            if baz is not None:
                pulumi.set(self, "baz", baz)
            if nested is not None:
                pulumi.set(self, "nested", nested)

        @property
        @pulumi.getter
        def foo(self) -> Optional[pulumi.Input[str]]:
            return pulumi.get(self, "foo")

        @property
        @pulumi.getter
        def bar(self) -> Optional[pulumi.Input[Sequence[pulumi.Input[str]]]]:
            return pulumi.get(self, "bar")

        @property
        @pulumi.getter
        def baz(self) -> Optional[pulumi.Input[Mapping[str, pulumi.Input[str]]]]:
            return pulumi.get(self, "baz")

        @property
        @pulumi.getter
        def nested(self) -> Optional[pulumi.Input[pulumi.InputType['NestedArgs']]]:
            return pulumi.get(self, "nested")

    @pulumi.input_type
    class NestedArgs:
        def __init__(self, *,
                     hello: Optional[pulumi.Input[str]] = None):
            if hello is not None:
                pulumi.set(self, "hello", hello)

        @property
        @pulumi.getter
        def hello(self) -> Optional[pulumi.Input[str]]:
            return pulumi.get(self, "hello")

    @pulumi_test
    async def test_unwrap_input_type(self):
        x = Output.from_input(OutputFromInputTests.FooArgs(foo=Output.from_input("bar")))
        x_val = cast(OutputFromInputTests.FooArgs, await x.future())
        self.assertIsInstance(x_val, OutputFromInputTests.FooArgs)
        self.assertEqual(x_val.foo, "bar")

    @pulumi_test
    async def test_unwrap_input_type_list(self):
        x = Output.from_input(OutputFromInputTests.FooArgs(bar=["a", Output.from_input("b")]))
        x_val = cast(OutputFromInputTests.FooArgs, await x.future())
        self.assertIsInstance(x_val, OutputFromInputTests.FooArgs)
        self.assertEqual(x_val.bar, ["a", "b"])

    @pulumi_test
    async def test_unwrap_input_type_dict(self):
        x = Output.from_input(OutputFromInputTests.FooArgs(baz={"hello": Output.from_input("world")}))
        x_val = cast(OutputFromInputTests.FooArgs, await x.future())
        self.assertIsInstance(x_val, OutputFromInputTests.FooArgs)
        self.assertEqual(x_val.baz, {"hello": "world"})

    @pulumi_test
    async def test_unwrap_input_type_nested(self):
        nested = OutputFromInputTests.NestedArgs(hello=Output.from_input("world"))
        x = Output.from_input(OutputFromInputTests.FooArgs(nested=nested))
        x_val = cast(OutputFromInputTests.FooArgs, await x.future())
        self.assertIsInstance(x_val, OutputFromInputTests.FooArgs)
        self.assertIsInstance(x_val.nested, OutputFromInputTests.NestedArgs)
        self.assertEqual(x_val.nested.hello, "world")

class Obj:
    def __init__(self, x: str):
        self.x = x

class OutputHoistingTests(unittest.TestCase):
    @pulumi_test
    async def test_item(self):
        o = Output.from_input([1,2,3])
        x = o[0]
        x_val = await x.future()
        self.assertEqual(x_val, 1)

    @pulumi_test
    async def test_attr(self):
        o = Output.from_input(Obj("hello"))
        x = o.x
        x_val = await x.future()
        self.assertEqual(x_val, "hello")

    @pulumi_test
    async def test_no_iter(self):
        x = Output.from_input([1,2,3])
        with self.assertRaises(TypeError):
            for i in x:
                print(i)
