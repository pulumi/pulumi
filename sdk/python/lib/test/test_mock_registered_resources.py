import pulumi
from pulumi.runtime import mocks
from pulumi.runtime import settings
import pytest
from copy import deepcopy
from typing import Optional


class MyMocks(mocks.Mocks):
    def new_resource(self, args: mocks.MockResourceArgs):
        return f"{args.name}_id", args.inputs

    def call(self, args: mocks.MockCallArgs):
        return {}, None


@pytest.fixture
def setup_mocks():
    monitor = mocks.MockMonitor(MyMocks())
    mocks.set_mocks(MyMocks(), monitor=monitor)
    try:
        yield monitor
    finally:
        settings.configure(deepcopy(settings.SETTINGS))


@pulumi.runtime.test
async def test_mock_registered_resources(setup_mocks: mocks.MockMonitor):
    class Component(pulumi.ComponentResource):
        def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
            super().__init__(name, "test:index:Component", opts)

    class Custom(pulumi.CustomResource):
        def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
            super().__init__(name, "test:index:Custom", opts)

    component = Component("component")
    custom = Custom("custom")

    component_urn = await component.urn.future()
    custom_urn = await custom.urn.future()

    registrations = setup_mocks.get_registered_resources()
    assert component_urn in registrations
    assert custom_urn in registrations


@pulumi.runtime.test
async def test_mock_exists_resource(setup_mocks: mocks.MockMonitor):
    class Custom(pulumi.CustomResource):
        def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
            super().__init__("test:index:Custom", name, {}, opts)

    await Custom("existing").id.future()

    exists = pulumi.runtime.exists_resource(
        None, "test:index:Custom", "existing_id", {}, pulumi.ResourceOptions()
    )
    assert await exists.is_known()
    assert await exists.future() is True

    missing = pulumi.runtime.exists_resource(
        None, "test:index:Custom", "some-id", {}, pulumi.ResourceOptions()
    )
    assert await missing.is_known()
    assert await missing.future() is False
