# Copyright 2026, Pulumi Corporation.
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

from typing import Optional

import pytest

import pulumi
from pulumi.runtime import mocks


class MyMocks(mocks.Mocks):
    def new_resource(self, args: mocks.MockResourceArgs):
        return f"{args.name}_id", dict(args.inputs)

    def call(self, args: mocks.MockCallArgs):
        return dict(args.args)


class TransformAwareMocks(mocks.Mocks):
    def __init__(self):
        self.stack_transforms = []
        self.stack_invoke_transforms = []

    def register_transform(self, transform: pulumi.ResourceTransform):
        self.stack_transforms.append(transform)

    def register_invoke_transform(self, transform: pulumi.InvokeTransform):
        self.stack_invoke_transforms.append(transform)

    def new_resource(self, args: mocks.MockResourceArgs):
        props = dict(args.inputs)
        for transform in [*args.transforms, *self.stack_transforms]:
            result = transform(
                pulumi.ResourceTransformArgs(
                    custom=args.custom or False,
                    type_=args.typ,
                    name=args.name,
                    props=props,
                    opts=pulumi.ResourceOptions(),
                )
            )
            if result is not None:
                props = dict(result.props)
        return f"{args.name}_id", props

    def call(self, args: mocks.MockCallArgs):
        call_args = dict(args.args)
        for transform in self.stack_invoke_transforms:
            result = transform(
                pulumi.InvokeTransformArgs(
                    token=args.token,
                    args=call_args,
                    opts=pulumi.InvokeOptions(),
                )
            )
            if result is not None:
                call_args = dict(result.args)
        return call_args


class MyResource(pulumi.CustomResource):
    def __init__(
        self,
        name: str,
        props: Optional[dict] = None,
        opts: Optional[pulumi.ResourceOptions] = None,
    ):
        super().__init__("test:index:MyResource", name, props or {}, opts)


@pytest.fixture
def setup_mocks():
    mocks.set_mocks(MyMocks())


@pytest.fixture
def setup_transform_aware_mocks():
    monitor = mocks.MockMonitor(TransformAwareMocks())
    mocks.set_mocks(TransformAwareMocks(), monitor=monitor)
    yield monitor


def tags_transform(args: pulumi.ResourceTransformArgs):
    props = dict(args.props)
    props["tags"] = {**(props.get("tags") or {}), "foo": "bar"}
    return pulumi.ResourceTransformResult(props, args.opts)


def test_transforms_are_not_run_by_the_mock_monitor(setup_mocks):
    pulumi.runtime.register_resource_transform(tags_transform)

    @pulumi.runtime.test
    async def check():
        res = MyResource(
            "res",
            {"tags": {"group": "webservers"}},
            opts=pulumi.ResourceOptions(transforms=[tags_transform]),
        )
        tags = await res.tags.future()
        assert tags == {"group": "webservers"}

    check()


@pulumi.runtime.test
async def test_register_resource_transform_in_loop(setup_mocks):
    pulumi.runtime.register_resource_transform(tags_transform)


def test_registered_transforms_are_exposed(setup_transform_aware_mocks):
    monitor = setup_transform_aware_mocks

    def other_transform(args: pulumi.ResourceTransformArgs):
        return None

    pulumi.runtime.register_resource_transform(tags_transform)
    pulumi.runtime.register_resource_transform(other_transform)

    assert monitor.get_registered_transforms() == [tags_transform, other_transform]


def test_manually_applying_stack_transforms(setup_transform_aware_mocks):
    pulumi.runtime.register_resource_transform(tags_transform)

    @pulumi.runtime.test
    async def check():
        res = MyResource("res", {"tags": {"group": "webservers"}})
        tags = await res.tags.future()
        assert tags == {"group": "webservers", "foo": "bar"}

    check()


def test_manually_applying_resource_transforms(setup_transform_aware_mocks):
    @pulumi.runtime.test
    async def check():
        res = MyResource(
            "res",
            {"tags": {"group": "webservers"}},
            opts=pulumi.ResourceOptions(transforms=[tags_transform]),
        )
        tags = await res.tags.future()
        assert tags == {"group": "webservers", "foo": "bar"}

    check()


def test_manually_applying_invoke_transforms(setup_transform_aware_mocks):
    def transform(args: pulumi.InvokeTransformArgs):
        new_args = dict(args.args)
        new_args["extra"] = "added"
        return pulumi.InvokeTransformResult(new_args, args.opts)

    pulumi.runtime.register_invoke_transform(transform)

    result = pulumi.runtime.invoke("test:index:MyFunction", {"orig": "value"}).value
    assert result == {"orig": "value", "extra": "added"}
