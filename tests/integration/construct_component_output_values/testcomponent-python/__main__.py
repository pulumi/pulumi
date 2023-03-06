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

from typing import Any, Mapping, Optional
import sys

import pulumi
import pulumi.provider as provider


@pulumi.input_type
class BarArgs:
    def __init__(__self__, *,
                 tags: Optional[pulumi.Input[Mapping[str, pulumi.Input[str]]]] = None):
        if tags is not None:
            pulumi.set(__self__, "tags", tags)

    @property
    @pulumi.getter
    def tags(self) -> Optional[pulumi.Input[Mapping[str, pulumi.Input[str]]]]:
        return pulumi.get(self, "tags")

    @tags.setter
    def tags(self, value: Optional[pulumi.Input[Mapping[str, pulumi.Input[str]]]]):
        pulumi.set(self, "tags", value)


@pulumi.input_type
class FooArgs:
    def __init__(__self__, *,
                 something: Optional[pulumi.Input[str]] = None):
        if something is not None:
            pulumi.set(__self__, "something", something)

    @property
    @pulumi.getter
    def something(self) -> Optional[pulumi.Input[str]]:
        return pulumi.get(self, "something")

    @something.setter
    def something(self, value: Optional[pulumi.Input[str]]):
        pulumi.set(self, "something", value)


@pulumi.input_type
class ComponentArgs:
    def __init__(__self__, *,
                 bar: Optional[pulumi.Input['BarArgs']] = None,
                 foo: Optional['FooArgs'] = None):
        if bar is not None:
            pulumi.set(__self__, "bar", bar)
        if foo is not None:
            pulumi.set(__self__, "foo", foo)

    @property
    @pulumi.getter
    def bar(self) -> Optional[pulumi.Input['BarArgs']]:
        return pulumi.get(self, "bar")

    @bar.setter
    def bar(self, value: Optional[pulumi.Input['BarArgs']]):
        pulumi.set(self, "bar", value)

    @property
    @pulumi.getter
    def foo(self) -> Optional['FooArgs']:
        return pulumi.get(self, "foo")

    @foo.setter
    def foo(self, value: Optional['FooArgs']):
        pulumi.set(self, "foo", value)


class Component(pulumi.ComponentResource):
    def __init__(self,
                 resource_name: str,
                 args: Optional[ComponentArgs] = None,
                 opts: Optional[pulumi.ResourceOptions] = None) -> None:
        super().__init__("testcomponent:index:Component", resource_name, args, opts)

        assert args.foo is not None, "expected args.foo to not be None"
        assert not isinstance(args.foo, pulumi.Output), "expected args.foo not to be an instance of pulumi.Output"
        assert args.foo.something == "hello", \
            f'expected args.foo.something to equal "hello" but got "{args.foo.something}"'

        assert args.bar is not None, "expected args.bar to not be None"
        assert not isinstance(args.bar, pulumi.Output), "expected args.bar not to be an instance of pulumi.Output"
        assert args.bar.tags is not None, "expected args.bar.tags to not be None"
        assert not isinstance(args.bar.tags, pulumi.Output), \
            "expected args.bar.tags not to be an instance of pulumi.Output"
        assert args.bar.tags["a"] == "world", \
            f'expected args.bar.tags["a"] to equal "world" but got "{args.bar.tags["a"]}"'
        assert isinstance(args.bar.tags["b"], pulumi.Output), 'expected args.bar.tags["b"] to be an instance of pulumi.Output'

        def validate_b(v: str):
            assert v == "shh", f'expected args.bar.tags["b"] to equal "shh" but got: "{v}"'
        assert args.bar.tags["b"].apply(validate_b)


class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super().__init__(Provider.VERSION)

    def construct(self, name: str, resource_type: str, inputs: pulumi.Inputs,
                  options: Optional[pulumi.ResourceOptions] = None) -> provider.ConstructResult:

        if resource_type != "testcomponent:index:Component":
            raise Exception(f"unknown resource type {resource_type}")

        component = Component(name, opts=options, args=ComponentArgs(
            foo=FooArgs(**inputs["foo"]) if "foo" in inputs else None,
            bar=BarArgs(**inputs["bar"]) if "bar" in inputs else None,
        ))

        return provider.ConstructResult(urn=component.urn, state={})


if __name__ == "__main__":
    provider.main(Provider(), sys.argv[1:])
