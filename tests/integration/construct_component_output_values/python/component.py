# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import pulumi
from typing import Mapping, Optional


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
    def __init__(__self__,
                 resource_name: str,
                 args: Optional[ComponentArgs] = None,
                 opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__('testcomponent:index:Component', resource_name, args, opts, remote=True)
