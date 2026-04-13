import asyncio
import importlib.metadata
import importlib.util
import sys
import types
from copy import deepcopy
from pathlib import Path

import pytest

from pulumi.runtime import settings
from pulumi.runtime.settings import Settings


class _FakeVersion:
    def __init__(self, major=2, minor=0, patch=0, prerelease=None):
        self.release = (major, minor, patch)
        self.pre_tag = None
        self.pre = None
        self.dev = None
        self.local = prerelease

    @classmethod
    def parse(cls, _):
        return cls()

    def __str__(self):
        return ".".join(str(part) for part in self.release)


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


def _load_generated_utilities(monkeypatch):
    semver = types.ModuleType("semver")
    semver.VersionInfo = _FakeVersion
    monkeypatch.setitem(sys.modules, "semver", semver)

    parver = types.ModuleType("parver")
    parver.Version = _FakeVersion
    monkeypatch.setitem(sys.modules, "parver", parver)

    monkeypatch.setattr(importlib.metadata, "version", lambda _: "2.0.0")

    root = Path(__file__).parents[5]
    utilities = (
        root
        / "sdk"
        / "python"
        / "cmd"
        / "pulumi-language-python"
        / "testdata"
        / "toml"
        / "published"
        / "sdks"
        / "subpackage-2.0.0"
        / "pulumi_subpackage"
        / "_utilities.py"
    )
    spec = importlib.util.spec_from_file_location("pulumi_subpackage._utilities_test", utilities)
    assert spec and spec.loader

    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


@pytest.mark.parametrize("first_ref,second_ref", [("ref-1", "ref-2")])
def test_generated_package_refs_are_reset_per_deployment_settings(monkeypatch, first_ref, second_ref):
    old_settings = deepcopy(settings.SETTINGS)
    utilities = _load_generated_utilities(monkeypatch)

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
