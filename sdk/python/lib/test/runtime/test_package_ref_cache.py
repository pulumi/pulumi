import asyncio
import sys
import types
from copy import deepcopy

import pytest

import pulumi
from pulumi.runtime import settings
from pulumi.runtime.settings import Settings
from pulumi.runtime.proto import resource_pb2


class _FakeMonitor:
    def __init__(self, ref):
        self.ref = ref
        self.known_refs = set()
        self.register_package_calls = 0

    def RegisterPackage(self, _):
        self.register_package_calls += 1
        self.known_refs.add(self.ref)
        return types.SimpleNamespace(ref=self.ref)

    def assert_known_package_ref(self, ref):
        if ref not in self.known_refs:
            raise Exception(f"unknown provider package '{ref}'")


def _load_generated_utilities():
    source = '''
import asyncio
import base64
import pulumi
import pulumi.runtime
from pulumi.runtime.proto import resource_pb2

def get_version():
    return "2.0.0"

def get_plugin_download_url():
    return None

_package_lock = asyncio.Lock()
_package_key = ("subpackage", get_version(), "parameterized", "1.2.3", "SGVsbG9Xb3JsZA==")

async def get_package():
    _package_ref = await pulumi.runtime.settings.get_package_ref(_package_key)
    if _package_ref is ...:
        async with _package_lock:
            _package_ref = await pulumi.runtime.settings.get_package_ref(_package_key)
            if _package_ref is ...:
                monitor = pulumi.runtime.settings.get_monitor()
                parameterization = resource_pb2.Parameterization(
                    name="subpackage",
                    version=get_version(),
                    value=base64.b64decode("SGVsbG9Xb3JsZA=="),
                )
                registerPackageResponse = monitor.RegisterPackage(
                    resource_pb2.RegisterPackageRequest(
                        name="parameterized",
                        version="1.2.3",
                        download_url=get_plugin_download_url(),
                        parameterization=parameterization,
                    ))
                _package_ref = registerPackageResponse.ref
                await pulumi.runtime.settings.set_package_ref(_package_key, _package_ref)
    if _package_ref is None or _package_ref is ...:
        raise Exception("The Pulumi CLI does not support parameterization. Please update the Pulumi CLI.")
    return _package_ref
'''

    module = types.ModuleType("pulumi_subpackage._utilities_test")
    module.__dict__.update({
        "asyncio": asyncio,
        "base64": __import__("base64"),
        "pulumi": pulumi,
        "resource_pb2": resource_pb2,
    })
    sys.modules[module.__name__] = module
    exec(source, module.__dict__)
    return module


@pytest.mark.parametrize("first_ref,second_ref", [("ref-1", "ref-2")])
def test_generated_package_refs_are_reset_per_deployment_settings(first_ref, second_ref):
    old_settings = deepcopy(settings.SETTINGS)
    utilities = _load_generated_utilities()

    try:
        first_monitor = _FakeMonitor(first_ref)
        settings.configure(Settings("project", "stack", monitor=first_monitor))
        settings.SETTINGS.feature_support["parameterization"] = True
        ref = asyncio.run(utilities.get_package())
        first_monitor.assert_known_package_ref(ref)

        second_monitor = _FakeMonitor(second_ref)
        settings.configure(Settings("project", "stack", monitor=second_monitor))
        settings.SETTINGS.feature_support["parameterization"] = True
        ref = asyncio.run(utilities.get_package())
        second_monitor.assert_known_package_ref(ref)

        assert first_monitor.register_package_calls == 1
        assert second_monitor.register_package_calls == 1
        assert ref == second_ref
    finally:
        settings.configure(old_settings)


def test_public_package_ref_apis_return_none_without_parameterization_support():
    old_settings = deepcopy(settings.SETTINGS)
    package_key = ("parameterized", "1.0.0")

    try:
        settings.configure(Settings("project", "stack"))
        assert asyncio.run(settings.get_package_ref(package_key)) is None
        asyncio.run(settings.set_package_ref(package_key, "ref-1"))
        assert asyncio.run(settings.get_package_ref(package_key)) is None
    finally:
        settings.configure(old_settings)
