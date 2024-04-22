# Copyright 2016-2022, Pulumi Corporation.
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
import json
import unittest
from typing import Mapping, Optional, Sequence, cast

from pulumi.runtime import rpc, rpc_manager, settings

import pulumi
from pulumi import Output


def pulumi_test(coro):
    wrapped = pulumi.runtime.test(coro)

    def wrapper(*args, **kwargs):
        settings.configure(settings.Settings("project", "stack"))
        rpc._RESOURCE_PACKAGES.clear()
        rpc._RESOURCE_MODULES.clear()

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
    async def test_unwrap_dict_output_key(self):
        x = Output.from_input({Output.from_input("hello"): Output.from_input("world")})
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
        o1 = {
            "a": {
                "a": {
                    "a": {
                        "a": {
                            "a": {
                                "a": {
                                    "a": {
                                        "a": {"a": {"a": {"a": Output.from_input("a")}}}
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
        o2 = {
            "a": {
                "a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": {"a": "a"}}}}}}}}}
            }
        }
        x = Output.from_input(o1)
        x_val = await x.future()
        self.assertEqual(x_val, o2)

    @pulumi_test
    async def test_unwrap_empty_tuple(self):
        x = Output.from_input(())
        x_val = await x.future()
        self.assertEqual(x_val, ())

    @pulumi_test
    async def test_unwrap_tuple(self):
        x = Output.from_input(("hello", Output.from_input("world")))
        x_val = await x.future()
        self.assertEqual(x_val, ("hello", "world"))

    @pulumi_test
    async def test_unwrap_tuple_tuple(self):
        x = Output.from_input(("hello", ("foo", Output.from_input("bar"))))
        x_val = await x.future()
        self.assertEqual(x_val, ("hello", ("foo", "bar")))

    @pulumi.input_type
    class FooArgs:
        def __init__(
            self,
            *,
            foo: Optional[pulumi.Input[str]] = None,
            bar: Optional[pulumi.Input[Sequence[pulumi.Input[str]]]] = None,
            baz: Optional[pulumi.Input[Mapping[str, pulumi.Input[str]]]] = None,
            nested: Optional[pulumi.Input[pulumi.InputType["NestedArgs"]]] = None
        ):
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
        def nested(self) -> Optional[pulumi.Input[pulumi.InputType["NestedArgs"]]]:
            return pulumi.get(self, "nested")

    @pulumi.input_type
    class NestedArgs:
        def __init__(self, *, hello: Optional[pulumi.Input[str]] = None):
            if hello is not None:
                pulumi.set(self, "hello", hello)

        @property
        @pulumi.getter
        def hello(self) -> Optional[pulumi.Input[str]]:
            return pulumi.get(self, "hello")

    @pulumi_test
    async def test_unwrap_input_type(self):
        x = Output.from_input(
            OutputFromInputTests.FooArgs(foo=Output.from_input("bar"))
        )
        x_val = cast(OutputFromInputTests.FooArgs, await x.future())
        self.assertIsInstance(x_val, OutputFromInputTests.FooArgs)
        self.assertEqual(x_val.foo, "bar")

    @pulumi_test
    async def test_unwrap_input_type_list(self):
        x = Output.from_input(
            OutputFromInputTests.FooArgs(bar=["a", Output.from_input("b")])
        )
        x_val = cast(OutputFromInputTests.FooArgs, await x.future())
        self.assertIsInstance(x_val, OutputFromInputTests.FooArgs)
        self.assertEqual(x_val.bar, ["a", "b"])

    @pulumi_test
    async def test_unwrap_input_type_dict(self):
        x = Output.from_input(
            OutputFromInputTests.FooArgs(baz={"hello": Output.from_input("world")})
        )
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

    @pulumi.input_type
    class EmptyArgs:
        pass

    @pulumi_test
    async def test_unwrap_empty_input_type(self):
        x = Output.from_input(OutputFromInputTests.EmptyArgs())
        x_val = cast(OutputFromInputTests.EmptyArgs, await x.future())
        self.assertIsInstance(x_val, OutputFromInputTests.EmptyArgs)


class Obj:
    def __init__(self, x: str):
        self.x = x


class OutputHoistingTests(unittest.TestCase):
    @pulumi_test
    async def test_item(self):
        o = Output.from_input([1, 2, 3])
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
        x = Output.from_input([1, 2, 3])
        with self.assertRaises(TypeError):
            for i in x:
                print(i)


class OutputStrTests(unittest.TestCase):
    @pulumi_test
    async def test_str(self):
        o = Output.from_input(1)
        self.assertEqual(
            str(o),
            """Calling __str__ on an Output[T] is not supported.

To get the value of an Output[T] as an Output[str] consider:
1. o.apply(lambda v: f"prefix{v}suffix")

See https://www.pulumi.com/docs/concepts/inputs-outputs for more details.
This function may throw in a future version of Pulumi.""",
        )


class OutputApplyTests(unittest.TestCase):

    async def test_apply_always_sets_is_secret_and_is_known(self):
        """Regressing a convoluted situation where apply created an Output
        with incomplete is_secret, is_known future, manifesting in
        program hangs when those futures were awaited.

        To reproduce this, a synthetic output is needed with is_known
        set to a Future completing exceptionally. Perhaps it would one
        day be possible to make it invalid for is_known to enter this
        state.

        """

        @pulumi_test
        async def test():
            bad = asyncio.Future()
            bad.set_exception(Exception("!"))
            ok = asyncio.Future()
            ok.set_result("ok")
            bad_output = Output(resources=set(), future=ok, is_known=bad)
            test_output = Output.from_input("anything").apply(lambda _: bad_output)
            self.assertEqual(await test_output.is_secret(), False)
            self.assertEqual(await test_output.is_known(), False)

        with self.assertRaises(Exception):
            # The overall test will fail with ! because pulumi_test awaits all outputs
            test()


class OutputAllTests(unittest.TestCase):
    @pulumi_test
    async def test_args(self):
        o1 = Output.from_input(1)
        o2 = Output.from_input("hi")
        x = Output.all(o1, o2)
        x_val = await x.future()
        self.assertEqual(x_val, [1, "hi"])

    @pulumi_test
    async def test_kwargs(self):
        o1 = Output.from_input(1)
        o2 = Output.from_input("hi")
        x = Output.all(x=o1, y=o2)
        x_val = await x.future()
        self.assertEqual(x_val, {"x": 1, "y": "hi"})


class OutputFormatTests(unittest.TestCase):
    @pulumi_test
    async def test_nothing(self):
        x = Output.format("blank format")
        x_val = await x.future()
        self.assertEqual(x_val, "blank format")

    @pulumi_test
    async def test_simple(self):
        i = Output.from_input(1)
        x = Output.format("{0}", i)
        x_val = await x.future()
        self.assertEqual(x_val, "1")

    @pulumi_test
    async def test_args_and_kwags(self):
        i = Output.from_input(1)
        s = Output.from_input("hi")
        x = Output.format("{0}, {s}", i, s=s)
        x_val = await x.future()
        self.assertEqual(x_val, "1, hi")


class OutputJsonDumpsTests(unittest.TestCase):
    @pulumi_test
    async def test_basic(self):
        i = Output.from_input([0, 1])
        x = Output.json_dumps(i)
        self.assertEqual(await x.future(), "[0, 1]")
        self.assertEqual(await x.is_secret(), False)
        self.assertEqual(await x.is_known(), True)

    # from_input handles _most_ nested outputs, so we need to use user defined types to "work around"
    # that, which means we also need to use a custom encoder
    class CustomClass(object):
        def __init__(self, a, b):
            self.a = a
            self.b = b

    class CustomEncoder(json.JSONEncoder):
        def default(self, o):
            if isinstance(o, OutputJsonDumpsTests.CustomClass):
                return (o.a, o.b)
            return json.JSONEncoder.default(self, o)

    @pulumi_test
    async def test_nested(self):
        i = Output.from_input(
            OutputJsonDumpsTests.CustomClass(Output.from_input(0), Output.from_input(1))
        )
        x = Output.json_dumps(i, cls=OutputJsonDumpsTests.CustomEncoder)
        self.assertEqual(await x.future(), "[0, 1]")
        self.assertEqual(await x.is_secret(), False)
        self.assertEqual(await x.is_known(), True)

    @pulumi_test
    async def test_nested_unknown(self):
        future = asyncio.Future()
        future.set_result(None)
        is_known = asyncio.Future()
        is_known.set_result(False)
        unknown = Output(resources=set(), future=future, is_known=is_known)

        i = Output.from_input(
            OutputJsonDumpsTests.CustomClass(unknown, Output.from_input(1))
        )
        x = Output.json_dumps(i, cls=OutputJsonDumpsTests.CustomEncoder)
        self.assertEqual(await x.is_secret(), False)
        self.assertEqual(await x.is_known(), False)

    @pulumi_test
    async def test_nested_secret(self):
        future = asyncio.Future()
        future.set_result(0)
        future_true = asyncio.Future()
        future_true.set_result(True)
        inner = Output(
            resources=set(), future=future, is_known=future_true, is_secret=future_true
        )

        i = Output.from_input(
            OutputJsonDumpsTests.CustomClass(inner, Output.from_input(1))
        )
        x = Output.json_dumps(i, cls=OutputJsonDumpsTests.CustomEncoder)
        self.assertEqual(await x.future(), "[0, 1]")
        self.assertEqual(await x.is_secret(), True)
        self.assertEqual(await x.is_known(), True)

    @pulumi_test
    async def test_nested_dependencies(self):
        future = asyncio.Future()
        future.set_result(0)
        future_true = asyncio.Future()
        future_true.set_result(True)
        resource = object()
        inner = Output(resources=set([resource]), future=future, is_known=future_true)

        i = Output.from_input(
            OutputJsonDumpsTests.CustomClass(inner, Output.from_input(1))
        )
        x = Output.json_dumps(i, cls=OutputJsonDumpsTests.CustomEncoder)
        self.assertEqual(await x.future(), "[0, 1]")
        self.assertEqual(await x.is_secret(), False)
        self.assertEqual(await x.is_known(), True)
        self.assertIn(resource, await x.resources())

    @pulumi_test
    async def test_output_keys(self):
        i = {Output.from_input("hello"): Output.from_input(1)}
        x = Output.json_dumps(i)
        self.assertEqual(await x.future(), '{"hello": 1}')
        self.assertEqual(await x.is_secret(), False)
        self.assertEqual(await x.is_known(), True)


class OutputJsonLoadsTests(unittest.TestCase):
    @pulumi_test
    async def test_basic(self):
        i = Output.from_input("[0, 1]")
        x = Output.json_loads(i)
        self.assertEqual(await x.future(), [0, 1])
        self.assertEqual(await x.is_secret(), False)
        self.assertEqual(await x.is_known(), True)
