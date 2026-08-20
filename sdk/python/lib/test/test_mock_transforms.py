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


class MyResource(pulumi.CustomResource):
    def __init__(
        self,
        name: str,
        props: Optional[dict] = None,
        opts: Optional[pulumi.ResourceOptions] = None,
    ):
        super().__init__("test:index:MyResource", name, props or {}, opts)


class MyComponent(pulumi.ComponentResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("test:index:MyComponent", name, None, opts)


@pytest.fixture
def setup_mocks():
    mocks.set_mocks(MyMocks())


def tags_transform(args: pulumi.ResourceTransformArgs):
    props = dict(args.props)
    props["tags"] = {**(props.get("tags") or {}), "foo": "bar"}
    return pulumi.ResourceTransformResult(props, args.opts)


def append_order(marker: str):
    def transform(args: pulumi.ResourceTransformArgs):
        props = dict(args.props)
        props["order"] = [*(props.get("order") or []), marker]
        return pulumi.ResourceTransformResult(props, args.opts)

    return transform


def test_stack_transform(setup_mocks):
    seen = {}

    def transform(args: pulumi.ResourceTransformArgs):
        seen["type"] = args.type_
        seen["name"] = args.name
        seen["custom"] = args.custom
        return tags_transform(args)

    pulumi.runtime.register_resource_transform(transform)

    @pulumi.runtime.test
    async def check():
        res = MyResource("res", {"tags": {"group": "webservers"}})
        tags = await res.tags.future()
        assert tags == {"group": "webservers", "foo": "bar"}

    check()

    assert seen == {"type": "test:index:MyResource", "name": "res", "custom": True}


@pulumi.runtime.test
async def test_stack_transform_registered_in_loop(setup_mocks):
    pulumi.runtime.register_resource_transform(tags_transform)

    res = MyResource("res", {"tags": {"group": "webservers"}})
    tags = await res.tags.future()
    assert tags == {"group": "webservers", "foo": "bar"}


@pulumi.runtime.test
async def test_resource_transform(setup_mocks):
    res = MyResource(
        "res",
        {"tags": {"group": "webservers"}},
        opts=pulumi.ResourceOptions(transforms=[tags_transform]),
    )
    tags = await res.tags.future()
    assert tags == {"group": "webservers", "foo": "bar"}


@pulumi.runtime.test
async def test_async_resource_transform(setup_mocks):
    async def transform(args: pulumi.ResourceTransformArgs):
        return tags_transform(args)

    res = MyResource(
        "res",
        {"tags": {"group": "webservers"}},
        opts=pulumi.ResourceOptions(transforms=[transform]),
    )
    tags = await res.tags.future()
    assert tags == {"group": "webservers", "foo": "bar"}


@pulumi.runtime.test
async def test_transform_returning_none_leaves_resource_unchanged(setup_mocks):
    def transform(args: pulumi.ResourceTransformArgs):
        return None

    res = MyResource(
        "res",
        {"tags": {"group": "webservers"}},
        opts=pulumi.ResourceOptions(transforms=[transform]),
    )
    tags = await res.tags.future()
    assert tags == {"group": "webservers"}


def test_transform_order(setup_mocks):
    pulumi.runtime.register_resource_transform(append_order("stack1"))
    pulumi.runtime.register_resource_transform(append_order("stack2"))

    @pulumi.runtime.test
    async def check():
        parent = MyComponent(
            "parent",
            opts=pulumi.ResourceOptions(transforms=[append_order("parent")]),
        )
        child = MyResource(
            "child",
            {"order": []},
            opts=pulumi.ResourceOptions(
                parent=parent, transforms=[append_order("own")]
            ),
        )
        order = await child.order.future()
        assert order == ["own", "parent", "stack1", "stack2"]

    check()


def test_grandparent_transform(setup_mocks):
    @pulumi.runtime.test
    async def check():
        grandparent = MyComponent(
            "grandparent",
            opts=pulumi.ResourceOptions(transforms=[tags_transform]),
        )
        parent = MyComponent("parent", opts=pulumi.ResourceOptions(parent=grandparent))
        child = MyResource(
            "child",
            {"tags": {"group": "webservers"}},
            opts=pulumi.ResourceOptions(parent=parent),
        )
        tags = await child.tags.future()
        assert tags == {"group": "webservers", "foo": "bar"}

    check()


def test_invoke_transform(setup_mocks):
    def transform(args: pulumi.InvokeTransformArgs):
        new_args = dict(args.args)
        new_args["extra"] = "added"
        return pulumi.InvokeTransformResult(new_args, args.opts)

    pulumi.runtime.register_invoke_transform(transform)

    result = pulumi.runtime.invoke("test:index:MyFunction", {"orig": "value"}).value
    assert result == {"orig": "value", "extra": "added"}
