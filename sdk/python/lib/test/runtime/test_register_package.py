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

import asyncio

import pytest

from pulumi.runtime import settings
from pulumi.runtime.proto import resource_pb2
from pulumi.runtime.settings import Settings, register_package


class MockMonitor:
    def __init__(self, ref):
        self.ref = ref
        self.register_package_calls = 0

    def RegisterPackage(self, _req):
        self.register_package_calls += 1
        return resource_pb2.RegisterPackageResponse(ref=self.ref)


def configure_monitor(ref, supports_parameterization=True):
    monitor = MockMonitor(ref)
    s = Settings(project="project", stack="stack")
    s.monitor = monitor
    s.feature_support = {"parameterization": supports_parameterization}
    settings.configure(s)
    return monitor


BASE_ARGS = {
    "base_provider_name": "base",
    "base_provider_version": "1.2.3",
    "base_provider_download_url": "",
    "package_name": "mypackage",
    "package_version": "2.0.0",
    "base64_parameter": "aGVsbG8=",
}


@pytest.mark.asyncio
async def test_registers_and_caches_the_ref():
    monitor = configure_monitor("uuid-1")
    ref = await register_package(**BASE_ARGS)
    assert ref == "uuid-1"
    assert monitor.register_package_calls == 1


@pytest.mark.asyncio
async def test_returns_the_same_ref_for_the_same_args():
    monitor = configure_monitor("uuid-1")
    a = await register_package(**BASE_ARGS)
    b = await register_package(**BASE_ARGS)
    assert a == b
    assert monitor.register_package_calls == 1


@pytest.mark.asyncio
async def test_distinct_cache_entry_per_identifying_field():
    monitor = configure_monitor("uuid")
    await register_package(**BASE_ARGS)
    await register_package(**{**BASE_ARGS, "package_version": "3.0.0"})
    await register_package(**{**BASE_ARGS, "base64_parameter": "T3RoZXI="})
    await register_package(
        **{**BASE_ARGS, "base_provider_download_url": "https://example/x"}
    )
    # Each distinct set of identifying fields is registered separately.
    assert monitor.register_package_calls == 4


@pytest.mark.asyncio
async def test_isolates_refs_between_concurrent_deployments():
    # Refs should be scoped to each deployment (the coroutines have their own contextvars)
    async def deployment(ref):
        monitor = configure_monitor(ref)
        await asyncio.sleep(0)  # let the other deployment configure before we register
        first = await register_package(**BASE_ARGS)
        await asyncio.sleep(0)  # interleave again before reading back from the cache
        second = await register_package(**BASE_ARGS)
        return ref, monitor, first, second

    results = await asyncio.gather(deployment("uuid-A"), deployment("uuid-B"))
    for ref, monitor, first, second in results:
        assert first == ref  # registered against this deployment's own monitor
        assert second == ref  # cache hit
        assert monitor.register_package_calls == 1


@pytest.mark.asyncio
async def test_raises_when_parameterization_is_not_supported():
    configure_monitor("uuid", supports_parameterization=False)
    with pytest.raises(Exception, match="does not support parameterization"):
        await register_package(**BASE_ARGS)


@pytest.mark.asyncio
async def test_raises_when_there_is_no_monitor():
    s = Settings(project="project", stack="stack")
    s.monitor = None
    s.feature_support = {"parameterization": True}
    settings.configure(s)
    with pytest.raises(Exception, match="No monitor available"):
        await register_package(**BASE_ARGS)
