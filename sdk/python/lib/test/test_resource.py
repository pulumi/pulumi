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

from typing import Optional
import asyncio
import pytest

from pulumi.runtime import settings, mocks
from pulumi.runtime.proto import resource_pb2
import pulumi


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
def test_depends_on_outputs_works_in_presence_of_unknowns(dep_tracker):
    dep1 = MockResource(name='dep1')
    dep2 = MockResource(name='dep2')
    dep3 = MockResource(name='dep3')
    out = output_depending_on_resource(dep1, isKnown=True).apply(lambda _: dep2)
    out2 = output_depending_on_resource(dep3, isKnown=False).apply(lambda _: dep2)

    res = MockResource(name='res', opts=pulumi.ResourceOptions(depends_on=[out, out2]))

    def check(urns):
        (dep1_urn, dep2_urn, dep3_urn, res_urn) = urns
        res_deps = dep_tracker.dependencies[res_urn]

        assert dep1_urn in res_deps, "Failed to propagate indirect dependencies via depends_on in presence of unknowns"
        assert dep3_urn in res_deps, "Failed to propagate indirect dependencies via depends_on in presence of unknowns"
        assert dep2_urn in res_deps, "Failed to propagate direct dependencies via depends_on in presence of unknowns"

    return pulumi.Output.all(dep1.urn, dep2.urn, dep3.urn, res.urn).apply(check)


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
    old_settings = settings.SETTINGS

    mm = MinimalMocks()
    mocks.set_mocks(mm, preview=True)
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


class DependencyTrackingMonitorWrapper(object):

    def __init__(self, inner):
        self.inner = inner
        self.dependencies = {}

    def RegisterResource(self, req: resource_pb2.RegisterResourceRequest):
        resp = self.inner.RegisterResource(req)
        self.dependencies[resp.urn] = self.dependencies.get(resp.urn, set()) | set(req.dependencies)
        return resp

    def __getattr__(self, attr):
        return getattr(self.inner, attr)


class MockResource(pulumi.CustomResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__('python:test:MockResource', name, {}, opts)
