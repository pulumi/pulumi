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

"""Tests that the provider server handles overlapping and reentrant
Construct/Call requests concurrently, and that per-request runtime state
(settings, config) is isolated between overlapping requests.

A provider's Construct may legitimately block awaiting work that only
completes once *another* request reaches the same server: for example, a
component whose children include remote components served by this same
process, or a method call arriving mid-construct. Serializing requests
turns those shapes into deadlocks.
"""

import asyncio
from typing import Callable, Optional

import pytest

import pulumi
import pulumi.runtime.config
import pulumi.runtime.settings
from pulumi import Inputs
from pulumi.output import _OutputData
from pulumi.provider import CallResult, ConstructResult
from pulumi.provider.provider import Provider
from pulumi.provider.server import ProviderServicer
from pulumi.resource import ResourceOptions
from pulumi.runtime import proto

from ..grpc_stubs import provider_servicer_stub

# Generous gate timeout: only ever reached if the server wrongly serializes
# requests, in which case it converts a hang into a fast, diagnosable failure.
GATE_TIMEOUT = 10
# Timeout for waiting on events that should fire promptly.
WAIT_TIMEOUT = 5


def _deferred_state(release: asyncio.Event, read: Callable[[], object]):
    """Returns an Output that resolves, once `release` is set, to the value
    produced by `read()`.

    The resolver task is created in the calling construct's request context,
    so `read()` observes that request's context-local runtime state -- after
    other requests have had a chance to interleave.
    """
    data_future: asyncio.Future = asyncio.Future()

    async def _resolve():
        try:
            await asyncio.wait_for(release.wait(), timeout=GATE_TIMEOUT)
            data_future.set_result(
                _OutputData(
                    resources=set(), value=read(), is_known=True, is_secret=False
                )
            )
        except BaseException as e:  # noqa
            if not data_future.done():
                data_future.set_exception(e)

    asyncio.ensure_future(_resolve())
    return pulumi.Output._from_data(data_future)


class GateProvider(Provider):
    """A provider whose Construct responses block until released.

    Entering construct/call is observable via per-resource events, and each
    construct's state reports the runtime settings its request context saw.
    """

    def __init__(self):
        super().__init__("1.0.0")
        self.entered: dict[str, asyncio.Event] = {}
        self.release: dict[str, asyncio.Event] = {}
        self.call_entered = asyncio.Event()

    def gate(self, name: str) -> None:
        self.entered[name] = asyncio.Event()
        self.release[name] = asyncio.Event()

    def construct(
        self,
        name: str,
        resource_type: str,
        inputs: Inputs,
        options: Optional[ResourceOptions] = None,
    ) -> ConstructResult:
        self.entered[name].set()

        def read():
            return {
                "project": pulumi.runtime.settings.get_project(),
                "stack": pulumi.runtime.settings.get_stack(),
                "config": pulumi.runtime.config.get_config("test:key"),
            }

        return ConstructResult(
            urn=f"urn:pulumi:stack::project::{resource_type}::{name}",
            state={"info": _deferred_state(self.release[name], read)},
        )

    def call(self, token: str, args: Inputs) -> CallResult:
        self.call_entered.set()
        return CallResult(outputs={"ok": True})


def _construct_request(name: str, project: str, stack: str, value: str):
    return proto.ConstructRequest(
        type="test:index:Gate",
        name=name,
        project=project,
        stack=stack,
        config={"test:key": value},
    )


def _construct_base_request(name: str, project: str, stack: str, value: str):
    return proto.ConstructBaseRequest(
        type="test:index:Gate",
        name=name,
        urn=f"urn:pulumi:{stack}::{project}::test:index:Derived::{name}",
        most_derived_type="test:index:Derived",
        project=project,
        stack=stack,
        config={"test:key": value},
    )


@pytest.mark.asyncio
async def test_overlapping_constructs_are_not_serialized():
    provider = GateProvider()
    provider.gate("one")
    provider.gate("two")
    servicer = ProviderServicer(provider, [], "")

    async with provider_servicer_stub(servicer) as stub:
        t1 = asyncio.ensure_future(
            stub.Construct(_construct_request("one", "p1", "s1", "v1"))
        )
        await asyncio.wait_for(provider.entered["one"].wait(), WAIT_TIMEOUT)

        # While the first response is gated, a second Construct must still be
        # able to enter the provider. If requests are serialized this wait
        # times out.
        t2 = asyncio.ensure_future(
            stub.Construct(_construct_request("two", "p2", "s2", "v2"))
        )
        await asyncio.wait_for(provider.entered["two"].wait(), WAIT_TIMEOUT)

        provider.release["one"].set()
        provider.release["two"].set()
        r1, r2 = await asyncio.wait_for(asyncio.gather(t1, t2), WAIT_TIMEOUT)

    assert r1.state["info"]["project"] == "p1"
    assert r2.state["info"]["project"] == "p2"


@pytest.mark.asyncio
async def test_call_during_construct_is_not_serialized():
    provider = GateProvider()
    provider.gate("one")
    servicer = ProviderServicer(provider, [], "")

    async with provider_servicer_stub(servicer) as stub:
        t1 = asyncio.ensure_future(
            stub.Construct(_construct_request("one", "p1", "s1", "v1"))
        )
        await asyncio.wait_for(provider.entered["one"].wait(), WAIT_TIMEOUT)

        # A method call arriving while a Construct is in flight (e.g. a call
        # on a component mid-construction) must be served, not queued behind
        # the blocked Construct.
        t2 = asyncio.ensure_future(
            stub.Call(proto.CallRequest(tok="test:index:Gate/method"))
        )
        await asyncio.wait_for(provider.call_entered.wait(), WAIT_TIMEOUT)

        provider.release["one"].set()
        r1, r2 = await asyncio.wait_for(asyncio.gather(t1, t2), WAIT_TIMEOUT)

    assert r1.state["info"]["project"] == "p1"
    assert getattr(r2, "return")["ok"] is True


@pytest.mark.asyncio
async def test_construct_base_during_construct_is_not_serialized():
    provider = GateProvider()
    provider.gate("one")
    provider.gate("two")
    servicer = ProviderServicer(provider, [], "")

    async with provider_servicer_stub(servicer) as stub:
        t1 = asyncio.ensure_future(
            stub.Construct(_construct_request("one", "p1", "s1", "v1"))
        )
        await asyncio.wait_for(provider.entered["one"].wait(), WAIT_TIMEOUT)

        # A ConstructBase (a derived component adopting an existing URN) arriving
        # while a Construct is gated must be served concurrently -- reentrant base
        # chains route through this same server, so serializing would deadlock.
        t2 = asyncio.ensure_future(
            stub.ConstructBase(_construct_base_request("two", "p2", "s2", "v2"))
        )
        await asyncio.wait_for(provider.entered["two"].wait(), WAIT_TIMEOUT)

        provider.release["one"].set()
        provider.release["two"].set()
        r1, r2 = await asyncio.wait_for(asyncio.gather(t1, t2), WAIT_TIMEOUT)

    # Each request observed its own context-local runtime state.
    assert r1.state["info"]["project"] == "p1"
    assert r1.state["info"]["stack"] == "s1"
    assert r1.state["info"]["config"] == "v1"
    assert r2.state["info"]["project"] == "p2"
    assert r2.state["info"]["stack"] == "s2"
    assert r2.state["info"]["config"] == "v2"


@pytest.mark.asyncio
async def test_runtime_state_isolated_across_overlapping_constructs():
    provider = GateProvider()
    provider.gate("one")
    provider.gate("two")
    servicer = ProviderServicer(provider, [], "")

    async with provider_servicer_stub(servicer) as stub:
        t1 = asyncio.ensure_future(
            stub.Construct(_construct_request("one", "p1", "s1", "v1"))
        )
        await asyncio.wait_for(provider.entered["one"].wait(), WAIT_TIMEOUT)
        t2 = asyncio.ensure_future(
            stub.Construct(_construct_request("two", "p2", "s2", "v2"))
        )
        await asyncio.wait_for(provider.entered["two"].wait(), WAIT_TIMEOUT)

        # Both requests have fully configured their runtime state; each gated
        # reader now runs and must observe its own request's values. Were the
        # settings process-global, both would see the second request's.
        provider.release["one"].set()
        provider.release["two"].set()
        r1, r2 = await asyncio.wait_for(asyncio.gather(t1, t2), WAIT_TIMEOUT)

    assert r1.state["info"]["project"] == "p1"
    assert r1.state["info"]["stack"] == "s1"
    assert r1.state["info"]["config"] == "v1"
    assert r2.state["info"]["project"] == "p2"
    assert r2.state["info"]["stack"] == "s2"
    assert r2.state["info"]["config"] == "v2"
