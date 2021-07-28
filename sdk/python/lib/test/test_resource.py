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
import pytest
import unittest

from pulumi.resource import DependencyProviderResource
from pulumi.runtime import settings, mocks
from pulumi.runtime.proto import resource_pb2
import pulumi


T = TypeVar('T')


class DependencyProviderResourceTests(unittest.TestCase):
    def test_get_package(self):
        res = DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0")
        self.assertEqual("aws", res.package)


@pulumi.runtime.test
def test_depends_on_accepts_outputs(dep_tracker):
    dep1 = MockResource(name='dep1')
    dep2 = MockResource(name='dep2')
    out = output_depending_on_resource(dep1, isKnown=True).apply(lambda _: dep2)
    res = MockResource(name='res', opts=pulumi.ResourceOptions(depends_on=[out]))

    def check(urns):
        (dep1_urn, dep2_urn, res_urn) = urns
        res_deps = dep_tracker.dependencies[res_urn]
        assert dep1_urn in res_deps, "Failed to propagate indirect dependencies via depends_on"
        assert dep2_urn in res_deps, "Failed to propagate direct dependencies via depends_on"

    return pulumi.Output.all(dep1.urn, dep2.urn, res.urn).apply(check)


@pulumi.runtime.test
def test_depends_on_outputs_works_in_presence_of_unknowns(dep_tracker_preview):
    dep1 = MockResource(name='dep1')
    dep2 = MockResource(name='dep2')
    dep3 = MockResource(name='dep3')
    known = output_depending_on_resource(dep1, isKnown=True).apply(lambda _: dep2)
    unknown = output_depending_on_resource(dep2, isKnown=False).apply(lambda _: dep2)
    res = MockResource(name='res', opts=pulumi.ResourceOptions(depends_on=[known, unknown]))

    def check(urns):
        (dep1_urn, res_urn) = urns
        assert dep1_urn in dep_tracker_preview.dependencies[res_urn]

    return pulumi.Output.all(dep1.urn, res.urn).apply(check)


@pulumi.runtime.test
def test_depends_on_respects_top_level_implicit_dependencies(dep_tracker):
    dep1 = MockResource(name='dep1')
    dep2 = MockResource(name='dep2')
    out = output_depending_on_resource(dep1, isKnown=True).apply(lambda _: [dep2])
    res = MockResource(name='res', opts=pulumi.ResourceOptions(depends_on=out))

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
    dep: pulumi.Resource = MockResource(name='dep1')

    def check(i, urns):
        (dep_urn, res_urn) = urns

        if i == 0:
            assert dep_tracker.dependencies[res_urn] == set([])
        else:
            assert dep_urn in dep_tracker.dependencies[res_urn]

    def check_opts(i, name, opts):
        res = MockResource(name, opts)
        return pulumi.Output.all(dep.urn, res.urn).apply(lambda urns: check(i, urns))

    return pulumi.Output.all([
        check_opts(i, f'res{i}', opts)
        for i, opts
        in enumerate(depends_on_variations(dep))
    ])


def output_depending_on_resource(r: pulumi.Resource, isKnown: bool) -> pulumi.Output[None]:
    """Returns an output that depends on the given resource."""
    o = pulumi.Output.from_input(None)
    is_known_fut: asyncio.Future[bool] = asyncio.Future()
    is_known_fut.set_result(isKnown)

    return pulumi.Output(
        resources=set([r]),
        is_known=is_known_fut,
        future=o.future())


@pytest.fixture
def dep_tracker():
    for dt in build_dep_tracker():
        yield dt


@pytest.fixture
def dep_tracker_preview():
    for dt in build_dep_tracker(preview=True):
        yield dt


def build_dep_tracker(preview: bool=False):
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
        return [args.name + '_id', args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}


class DependencyTrackingMonitorWrapper:

    def __init__(self, inner):
        self.inner = inner
        self.dependencies = {}

    def RegisterResource(self, req: Any):
        resp = self.inner.RegisterResource(req)
        self.dependencies[resp.urn] = self.dependencies.get(resp.urn, set()) | set(req.dependencies)
        return resp

    def __getattr__(self, attr):
        return getattr(self.inner, attr)


class MockResource(pulumi.CustomResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__('python:test:MockResource', name, {}, opts)
