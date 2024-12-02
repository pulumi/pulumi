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

from typing import Optional, TypeVar, Awaitable, List, Any
import asyncio
import os
import unittest
import pytest

from pulumi.resource import DependencyProviderResource
from pulumi.runtime import settings, mocks
from pulumi.runtime.proto import resource_pb2
from pulumi import ResourceOptions
from pulumi.runtime.rpc import ERROR_ON_DEPENDENCY_CYCLES_VAR
import pulumi


T = TypeVar("T")


class DependencyProviderResourceTests(unittest.TestCase):
    def test_get_package(self):
        res = DependencyProviderResource(
            "urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0"
        )
        self.assertEqual("aws", res.package)


@pytest.fixture(autouse=True)
def clean_up_env_vars():
    try:
        del os.environ[ERROR_ON_DEPENDENCY_CYCLES_VAR]
    except KeyError:
        pass


@pulumi.runtime.test
def test_depends_on_accepts_outputs(dep_tracker):
    dep1 = MockResource(name="dep1")
    dep2 = MockResource(name="dep2")
    out = output_depending_on_resource(dep1, isKnown=True).apply(lambda _: dep2)
    res = MockResource(name="res", opts=pulumi.ResourceOptions(depends_on=[out]))

    def check(urns):
        (dep1_urn, dep2_urn, res_urn) = urns
        res_deps = dep_tracker.dependencies[res_urn]
        assert (
            dep1_urn in res_deps
        ), "Failed to propagate indirect dependencies via depends_on"
        assert (
            dep2_urn in res_deps
        ), "Failed to propagate direct dependencies via depends_on"

    return pulumi.Output.all(dep1.urn, dep2.urn, res.urn).apply(check)


@pulumi.runtime.test
def test_depends_on_outputs_works_in_presence_of_unknowns(dep_tracker_preview):
    dep1 = MockResource(name="dep1")
    dep2 = MockResource(name="dep2")
    dep3 = MockResource(name="dep3")
    known = output_depending_on_resource(dep1, isKnown=True).apply(lambda _: dep2)
    unknown = output_depending_on_resource(dep2, isKnown=False).apply(lambda _: dep2)
    res = MockResource(
        name="res", opts=pulumi.ResourceOptions(depends_on=[known, unknown])
    )

    def check(urns):
        (dep1_urn, res_urn) = urns
        assert dep1_urn in dep_tracker_preview.dependencies[res_urn]

    return pulumi.Output.all(dep1.urn, res.urn).apply(check)


@pulumi.runtime.test
def test_depends_on_respects_top_level_implicit_dependencies(dep_tracker):
    dep1 = MockResource(name="dep1")
    dep2 = MockResource(name="dep2")
    out = output_depending_on_resource(dep1, isKnown=True).apply(lambda _: [dep2])
    res = MockResource(name="res", opts=pulumi.ResourceOptions(depends_on=out))

    def check(urns):
        (dep1_urn, dep2_urn, res_urn) = urns
        assert set(dep_tracker.dependencies[res_urn]) == set([dep1_urn, dep2_urn])

    return pulumi.Output.all(dep1.urn, dep2.urn, res.urn).apply(check)


def promise(x: T) -> Awaitable[T]:
    fut: asyncio.Future[T] = asyncio.Future()
    fut.set_result(x)
    return fut


def out(x: T) -> pulumi.Output[T]:
    return pulumi.Output.from_input(x)


def depends_on_variations(dep: pulumi.Resource) -> List[pulumi.ResourceOptions]:
    return [
        pulumi.ResourceOptions(depends_on=None),
        pulumi.ResourceOptions(depends_on=dep),
        pulumi.ResourceOptions(depends_on=[dep]),
        pulumi.ResourceOptions(depends_on=promise(dep)),
        pulumi.ResourceOptions(depends_on=out(dep)),
        pulumi.ResourceOptions(depends_on=promise([dep])),
        pulumi.ResourceOptions(depends_on=out([dep])),
        pulumi.ResourceOptions(depends_on=promise([promise(dep)])),
        pulumi.ResourceOptions(depends_on=promise([out(dep)])),
        pulumi.ResourceOptions(depends_on=out([promise(dep)])),
        pulumi.ResourceOptions(depends_on=out([out(dep)])),
    ]


@pulumi.runtime.test
def test_depends_on_typing_variations(dep_tracker) -> None:
    dep: pulumi.Resource = MockResource(name="dep1")

    def check(i, urns):
        (dep_urn, res_urn) = urns

        if i == 0:
            assert dep_tracker.dependencies[res_urn] == set([])
        else:
            assert dep_urn in dep_tracker.dependencies[res_urn]

    def check_opts(i, name, opts):
        res = MockResource(name, opts)
        return pulumi.Output.all(dep.urn, res.urn).apply(lambda urns: check(i, urns))

    pulumi.Output.all(
        [
            check_opts(i, f"res{i}", opts)
            for i, opts in enumerate(depends_on_variations(dep))
        ]
    )


@pulumi.runtime.test
def test_depends_on_typechecks_sync():
    # https://github.com/pulumi/pulumi/issues/13917
    try:
        res = MockResource(
            name="res", opts=pulumi.ResourceOptions(depends_on=["hello"])
        )
        assert False, "should of failed"
    except TypeError as e:
        assert (
            str(e) == "'depends_on' was passed a value hello that was not a Resource."
        )


def test_depends_on_typechecks_async():
    if not hasattr(asyncio, "to_thread"):
        # Old versions of Python don't have asyncio.to_thread, just skip the test in that case.
        return

    @pulumi.runtime.test
    def test():
        # https://github.com/pulumi/pulumi/issues/13917
        dep = asyncio.to_thread(lambda: "goodbye")
        res = MockResource(name="res", opts=pulumi.ResourceOptions(depends_on=[dep]))

    try:
        test()
        assert False, "should of failed"
    except TypeError as e:
        assert (
            str(e) == "'depends_on' was passed a value goodbye that was not a Resource."
        )


@pulumi.runtime.test
def test_component_resource_propagates_provider() -> None:
    mocks.set_mocks(MinimalMocks())

    provider = pulumi.ProviderResource("test", "prov", {})
    component = pulumi.ComponentResource(
        "custom:foo:Component",
        "comp",
        opts=pulumi.ResourceOptions(provider=provider),
    )
    custom = pulumi.CustomResource(
        "test:index:Resource",
        "res",
        opts=pulumi.ResourceOptions(parent=component),
    )

    assert (
        provider == custom._provider
    ), "Failed to propagate provider to child resource"


@pulumi.runtime.test
def test_component_resource_propagates_providers_list() -> None:
    mocks.set_mocks(MinimalMocks())

    provider = pulumi.ProviderResource("test", "prov", {})
    component = pulumi.ComponentResource(
        "custom:foo:Component",
        "comp",
        opts=pulumi.ResourceOptions(providers=[provider]),
    )
    custom = pulumi.CustomResource(
        "test:index:Resource",
        "res",
        opts=pulumi.ResourceOptions(parent=component),
    )

    assert (
        provider == custom._provider
    ), "Failed to propagate provider to child resource"


def output_depending_on_resource(
    r: pulumi.Resource, isKnown: bool
) -> pulumi.Output[None]:
    """Returns an output that depends on the given resource."""
    o = pulumi.Output.from_input(None)
    is_known_fut: asyncio.Future[bool] = asyncio.Future()
    is_known_fut.set_result(isKnown)

    return pulumi.Output(resources=set([r]), is_known=is_known_fut, future=o.future())


@pytest.fixture
def dep_tracker():
    for dt in build_dep_tracker():
        yield dt


@pytest.fixture
def dep_tracker_preview():
    for dt in build_dep_tracker(preview=True):
        yield dt


def build_dep_tracker(preview: bool = False):
    old_settings = settings.SETTINGS
    mm = MinimalMocks()
    mocks.set_mocks(mm, preview=preview)
    dt = DependencyTrackingMonitorWrapper(settings.SETTINGS.monitor)
    settings.SETTINGS.monitor = dt

    try:
        yield dt
    finally:
        settings.configure(old_settings)


class MinimalMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}


class DependencyTrackingMonitorWrapper:
    def __init__(self, inner):
        self.inner = inner
        self.dependencies = {}

    def RegisterResource(self, req: Any):
        resp = self.inner.RegisterResource(req)
        self.dependencies[resp.urn] = self.dependencies.get(resp.urn, set()) | set(
            req.dependencies
        )
        return resp

    def __getattr__(self, attr):
        return getattr(self.inner, attr)


class MockResource(pulumi.CustomResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("python:test:MockResource", name, {}, opts)


class MergeResourceOptions(unittest.TestCase):
    def test_parent(self):
        opts1 = ResourceOptions()
        assert opts1.protect is None
        opts2 = ResourceOptions.merge(opts1, ResourceOptions(protect=True))
        assert opts2.protect is True
        opts3 = ResourceOptions.merge(opts2, ResourceOptions())
        assert opts3.protect is True


# Regression test for https://github.com/pulumi/pulumi/issues/12032
@pulumi.runtime.test
def test_parent_and_depends_on_are_the_same_12032():
    os.environ[ERROR_ON_DEPENDENCY_CYCLES_VAR] = "false"
    mocks.set_mocks(MinimalMocks())

    parent = pulumi.ComponentResource("pkg:index:first", "first")
    child = pulumi.ComponentResource(
        "pkg:index:second",
        "second",
        opts=pulumi.ResourceOptions(parent=parent, depends_on=[parent]),
    )

    # This would freeze before the fix.
    pulumi.CustomResource(
        "foo:bar:baz",
        "myresource",
        opts=pulumi.ResourceOptions(parent=child),
    )


# Regression test for https://github.com/pulumi/pulumi/issues/12736
@pulumi.runtime.test
def test_complex_parent_child_dependencies():
    os.environ[ERROR_ON_DEPENDENCY_CYCLES_VAR] = "false"
    mocks.set_mocks(MinimalMocks())

    class A(pulumi.ComponentResource):
        def __init__(self, name: str, opts=None):
            super().__init__("my:modules:A", name, {}, opts)
            self.b = B("a-b", opts=ResourceOptions(parent=self))
            self.c = C("a-c", opts=ResourceOptions(parent=self.b, depends_on=[self.b]))

    class B(pulumi.ComponentResource):
        def __init__(self, name: str, opts=None):
            super().__init__("my:modules:B", name, {}, opts)
            pulumi.CustomResource(
                "my:module:Child", "b-child", opts=ResourceOptions(parent=self)
            )

    class C(pulumi.ComponentResource):
        def __init__(self, name: str, opts=None):
            super().__init__("my:modules:C", name, {}, opts)
            pulumi.CustomResource(
                "my:module:Child", "c-child", opts=ResourceOptions(parent=self)
            )

    class D(pulumi.ComponentResource):
        def __init__(self, name: str, opts=None):
            super().__init__("my:modules:D", name, {}, opts)
            pulumi.CustomResource(
                "my:module:Child", "d-child", opts=ResourceOptions(parent=self)
            )

    a = A("a")

    D("d", opts=ResourceOptions(parent=a.b, depends_on=[a.b]))


# Regression test for https://github.com/pulumi/pulumi/issues/13997
def test_bad_component_super_call():
    class C(pulumi.ComponentResource):
        def __init__(self, name: str, arg: int, opts=None):
            super().__init__("my:module:C", name, arg, opts)

    @pulumi.runtime.test
    def test():
        C("test", 4, None)

    try:
        test()
        assert False, "should of failed"
    except TypeError as e:
        assert str(e) == "Expected resource properties to be a mapping"
