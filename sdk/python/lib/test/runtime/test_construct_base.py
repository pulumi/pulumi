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

"""Unit tests for base-class construction: the SDK's attach mode (a component
adopting an already-registered URN) and the `construct_base_resource` monitor
helper that drives it from the deriving side."""

import asyncio
from typing import Optional

import pytest

import pulumi
import pulumi.runtime
from pulumi import Inputs
from pulumi.output import _OutputData
from pulumi.provider import ConstructResult
from pulumi.provider.provider import Provider
from pulumi.provider.server import ProviderServicer
from pulumi.resource import _ConstructBaseOptions
from pulumi.runtime import mocks, proto, rpc, settings
from pulumi.runtime.proto import resource_pb2
from pulumi.runtime.sync_await import _sync_await

from ..grpc_stubs import provider_servicer_stub

_SPECIAL_SIG_KEY = "4dabf18193072939515e22adb298388d"
_OUTPUT_VALUE_SIG = "d0e6a833031e9bbcd3f4e8bde6ca49a4"


class _Mocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}


class _SpyMonitor(mocks.MockMonitor):
    """Records the monitor traffic attach mode must *not* produce for the base."""

    def __init__(self, mocks_impl):
        super().__init__(mocks_impl)
        self.registered_types: list[tuple[str, str]] = []
        self.register_outputs_urns: list[str] = []

    def RegisterResource(self, request):
        self.registered_types.append((request.type, request.name))
        return super().RegisterResource(request)

    def RegisterResourceOutputs(self, request):
        self.register_outputs_urns.append(request.urn)
        return super().RegisterResourceOutputs(request)


class _Child(pulumi.CustomResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("test:index:Child", name, {}, opts)


class _Base(pulumi.ComponentResource):
    """A component whose constructor body creates a child and registers outputs
    -- the shape a base implementation takes when run in attach mode."""

    child: _Child

    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        # The component's own token is a base token; when adopted it inherits the
        # most-derived URN, so children must parent to *that*, not to this token.
        super().__init__("pkgC:index:Base", name, {}, opts)
        self.child = _Child(f"{name}-child", pulumi.ResourceOptions(parent=self))
        self.register_outputs({"childUrn": self.child.urn})


@pytest.fixture
def spy_monitor():
    old_settings = settings.SETTINGS
    impl = _Mocks()
    monitor = _SpyMonitor(impl)
    mocks.set_mocks(impl, monitor=monitor)
    try:
        yield monitor
    finally:
        settings.configure(old_settings)


@pulumi.runtime.test
def test_attach_mode_adopts_urn(spy_monitor):
    adopt_urn = "urn:pulumi:stack::project::pkgB:index:Derived::my-derived"
    opts = pulumi.ResourceOptions()
    opts._construct_base = _ConstructBaseOptions(
        urn=adopt_urn, most_derived_type="pkgB:index:Derived"
    )

    base = _Base("my-derived", opts)

    assert base._adopted is True

    def check(args):
        base_urn, child_urn = args

        # The base adopts the URN directly: no fresh registration, no getResource.
        assert base_urn == adopt_urn

        # The child parents to the *adopted* URN, not to a URN derived from the
        # base component's own token -- this is the child-stability invariant.
        assert child_urn.startswith("urn:pulumi:stack::project::pkgB:index:Derived$")
        assert child_urn.endswith("::my-derived-child")

        # The base itself never registered, and its register_outputs was a no-op.
        assert ("pkgC:index:Base", "my-derived") not in spy_monitor.registered_types
        assert ("test:index:Child", "my-derived-child") in spy_monitor.registered_types
        assert adopt_urn not in spy_monitor.register_outputs_urns

    return pulumi.Output.all(base.urn, base.child.urn).apply(check)


class _SimpleBase(pulumi.ComponentResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("pkgC:index:Base", name, {}, opts)
        # No-op under attach mode; exercises the suppression path.
        self.register_outputs({})


class _AttachProvider(Provider):
    """A hand-written provider whose construct builds a real ComponentResource,
    exercising the full server ConstructBase -> attach-options -> adopt path."""

    def __init__(self):
        super().__init__("1.0.0")

    def construct(
        self,
        name: str,
        resource_type: str,
        inputs: Inputs,
        options: Optional[pulumi.ResourceOptions] = None,
    ) -> ConstructResult:
        comp = _SimpleBase(name, options)
        return ConstructResult(comp.urn, {"adopted": comp.urn})


@pytest.mark.asyncio
async def test_server_construct_base_adopts_urn():
    servicer = ProviderServicer(_AttachProvider(), [], "")
    adopt_urn = "urn:pulumi:s::p::pkgB:index:Derived::comp"

    async with provider_servicer_stub(servicer) as stub:
        resp = await stub.ConstructBase(
            proto.ConstructBaseRequest(
                type="pkgC:index:Base",
                name="comp",
                urn=adopt_urn,
                most_derived_type="pkgB:index:Derived",
                project="p",
                stack="s",
            )
        )

    # The server threaded the attach options through construct, so the component
    # adopted the derived URN in-process rather than registering its own.
    assert resp.state["adopted"] == adopt_urn


class _FakeBaseMonitor:
    """A minimal monitor exposing just the surface `construct_base_resource`
    touches: feature negotiation via GetDeploymentInfo, output-value support,
    and the ConstructBaseResource call itself."""

    def __init__(self, *, advertise: bool, state: Optional[dict] = None):
        self.advertise = advertise
        self.state = state or {}
        self.requests: list[resource_pb2.ConstructBaseResourceRequest] = []

    def GetDeploymentInfo(self, request):
        features = []
        if self.advertise:
            features.append(resource_pb2.RESOURCE_MONITOR_FEATURE_CONSTRUCT_BASE)
        return resource_pb2.DeploymentInfo(supportedFeatures=features)

    def SupportsFeature(self, request):
        return type("SupportsFeatureResponse", (object,), {"hasSupport": True})

    def ConstructBaseResource(self, request):
        self.requests.append(request)
        state = _sync_await(rpc.serialize_properties(self.state, {}))
        return resource_pb2.ConstructBaseResourceResponse(state=state)


def _configure(monitor) -> None:
    settings.configure(
        settings.Settings(
            project="project",
            stack="stack",
            monitor=monitor,
        )
    )


def _resolved_component(urn: str, type_: str) -> pulumi.ComponentResource:
    """A stand-in derived component whose URN is already resolved, isolating
    `construct_base_resource` from how the resource got registered."""

    class Derived(pulumi.ComponentResource):
        pass

    res = Derived.__new__(Derived)
    res._type = type_
    res._name = urn.split("::")[-1]
    res._providers = {}
    fut: asyncio.Future = asyncio.Future()
    fut.set_result(
        _OutputData(resources=set(), value=urn, is_known=True, is_secret=False)
    )
    res.__dict__["urn"] = pulumi.Output._from_data(fut)
    return res


@pytest.mark.asyncio
async def test_construct_base_resolves_outputs():
    monitor = _FakeBaseMonitor(advertise=True, state={"outA": "hello", "outB": 42})
    _configure(monitor)

    urn = "urn:pulumi:stack::project::pkgB:index:Derived::n"
    res = _resolved_component(urn, "pkgB:index:Derived")

    info = pulumi.runtime.BaseConstructInfo(version="1.2.3")
    pulumi.runtime.construct_base_resource(
        res,
        "pkgA:index:Foo",
        {"x": pulumi.Output.from_input("v")},
        info,
        ["outA", "outB"],
    )

    # Awaiting the seeded outputs drives the scheduled base construction.
    assert await res.__dict__["outA"].future() == "hello"
    assert await res.__dict__["outB"].future() == 42

    assert len(monitor.requests) == 1
    req = monitor.requests[0]
    assert req.urn == urn
    assert req.base_type == "pkgA:index:Foo"
    assert req.version == "1.2.3"

    # Inputs are serialized as rich output values (both RPC peers are new).
    x = req.inputs["x"]
    assert x[_SPECIAL_SIG_KEY] == _OUTPUT_VALUE_SIG
    assert x["value"] == "v"


@pytest.mark.asyncio
async def test_construct_base_ignores_unnamed_keys():
    # Only output_keys land on the instance; other returned properties are dropped.
    monitor = _FakeBaseMonitor(
        advertise=True, state={"outA": "hello", "extra": "ignored"}
    )
    _configure(monitor)

    res = _resolved_component(
        "urn:pulumi:stack::project::pkgB:index:Derived::n", "pkgB:index:Derived"
    )
    pulumi.runtime.construct_base_resource(
        res, "pkgA:index:Foo", {}, pulumi.runtime.BaseConstructInfo(), ["outA"]
    )

    assert await res.__dict__["outA"].future() == "hello"
    assert "extra" not in res.__dict__


@pytest.mark.asyncio
async def test_construct_base_overwrites_registration_placeholder():
    # An inherited output may already carry an unresolved placeholder cell left by
    # the derived registration (e.g. a property that is both input and output).
    # The base is authoritative for its own outputs, so its value must land.
    monitor = _FakeBaseMonitor(advertise=True, state={"outA": "from-base"})
    _configure(monitor)

    res = _resolved_component(
        "urn:pulumi:stack::project::pkgB:index:Derived::n", "pkgB:index:Derived"
    )
    placeholder = pulumi.Output.from_input("from-registration")
    res.__dict__["outA"] = placeholder

    pulumi.runtime.construct_base_resource(
        res, "pkgA:index:Foo", {}, pulumi.runtime.BaseConstructInfo(), ["outA"]
    )

    assert await res.__dict__["outA"].future() == "from-base"


@pytest.mark.asyncio
async def test_construct_base_unsupported_engine_errors():
    monitor = _FakeBaseMonitor(advertise=False)
    _configure(monitor)

    res = _resolved_component(
        "urn:pulumi:stack::project::pkgB:index:Derived::n", "pkgB:index:Derived"
    )
    pulumi.runtime.construct_base_resource(
        res, "pkgA:index:Foo", {}, pulumi.runtime.BaseConstructInfo(), ["outA"]
    )

    with pytest.raises(Exception) as exc:
        await res.__dict__["outA"].future()

    assert "requires a newer version of the Pulumi CLI" in str(exc.value)
    # The engine was never asked to construct the base.
    assert monitor.requests == []
