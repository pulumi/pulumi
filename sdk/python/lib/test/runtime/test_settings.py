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

from __future__ import annotations

from typing import cast

import pytest

from pulumi.runtime._monitor import _sync_to_async_monitor
from pulumi.runtime.mocks import AsyncMockMonitor, MockMonitor, Mocks, set_mocks
from pulumi.runtime.proto import resource_pb2
from pulumi.runtime.settings import ROOT, SETTINGS, _get_async_monitor, get_monitor


class _SimpleMocks(Mocks):
    def call(self, args):
        return {}, None

    def new_resource(self, args):
        return args.name + "_id", {}


# Capture the qualname of SyncToAsyncMonitorProxy once (each _sync_to_async_monitor
# call creates a fresh class, but all share the same __qualname__).
_SYNC_TO_ASYNC_PROXY_QUALNAME: str = type(
    _sync_to_async_monitor(cast(object, object()))  # type: ignore[arg-type]
).__qualname__


def _is_sync_to_async_wrapper(obj) -> bool:
    """Returns True when obj is a _sync_to_async_monitor proxy."""
    return type(obj).__qualname__ == _SYNC_TO_ASYNC_PROXY_QUALNAME


@pytest.fixture(autouse=True)
def _reset_settings():
    """Restore monitor and root-resource state between tests."""
    old_monitor = SETTINGS.monitor
    old_root = ROOT.get()
    yield
    SETTINGS.monitor = old_monitor
    ROOT.set(old_root)


class TestGetMonitor:
    def test_returns_none_when_no_monitor_configured(self):
        SETTINGS.monitor = None
        assert get_monitor() is None

    def test_returns_settings_monitor_directly(self):
        mock_mon = MockMonitor(_SimpleMocks())
        SETTINGS.monitor = mock_mon
        assert get_monitor() is mock_mon

    def test_sync_register_package_on_mock_monitor(self):
        """
        Mirrors the real-world usage pattern we currently have in generated
        provider SDKs, where _utilities.py calls get_monitor() and then calls
        RegisterPackage synchronously:

            monitor = get_monitor()
            response = monitor.RegisterPackage(RegisterPackageRequest(...))

        MockMonitor.RegisterPackage is synchronous; the response is the same
        fake ref that AsyncMockMonitor returns.
        """
        set_mocks(_SimpleMocks())
        monitor = get_monitor()
        assert monitor is not None

        request = resource_pb2.RegisterPackageRequest(
            name="mypkg",
            version="1.2.3",
        )
        response = monitor.RegisterPackage(request)  # type: ignore[attr-defined]

        # AsyncMockMonitor always returns "mock-uuid" for RegisterPackage.
        assert response.ref == "mock-uuid"

    def test_sync_register_package_with_mock_monitor_subclass(self):
        """A MockMonitor subclass passed explicitly also exposes sync RegisterPackage."""

        class _SubMon(MockMonitor):
            pass

        set_mocks(_SimpleMocks(), monitor=_SubMon(_SimpleMocks()))
        monitor = get_monitor()

        response = monitor.RegisterPackage(  # type: ignore[attr-defined]
            resource_pb2.RegisterPackageRequest(name="pkg", version="0.1.0")
        )
        assert response.ref == "mock-uuid"

    def test_sync_register_package_with_async_mock_monitor_passed(self):
        """AsyncMockMonitor wrapped by set_mocks; sync RegisterPackage still works."""
        async_mon = AsyncMockMonitor(_SimpleMocks())
        set_mocks(_SimpleMocks(), monitor=async_mon)
        monitor = get_monitor()
        assert isinstance(monitor, MockMonitor)

        response = monitor.RegisterPackage(  # type: ignore[attr-defined]
            resource_pb2.RegisterPackageRequest(name="pkg", version="0.1.0")
        )
        assert response.ref == "mock-uuid"

    def test_returns_arbitrary_monitor_unchanged(self):
        """An object that is not MockMonitor/AsyncMockMonitor is stored and returned as-is."""

        class _ArbitraryMonitor:
            pass

        arb = _ArbitraryMonitor()
        SETTINGS.monitor = arb
        assert get_monitor() is arb


class TestGetAsyncMonitor:
    def test_returns_none_when_no_monitor_configured(self):
        SETTINGS.monitor = None
        assert _get_async_monitor() is None

    def test_returns_async_mock_monitor_for_plain_mock_monitor(self):
        """MockMonitor.async_monitor() returns the inner AsyncMockMonitor."""
        set_mocks(_SimpleMocks())
        assert isinstance(_get_async_monitor(), AsyncMockMonitor)

    def test_returns_inner_async_mock_monitor_for_explicit_mock_monitor(self):
        mock_mon = MockMonitor(_SimpleMocks())
        SETTINGS.monitor = mock_mon
        async_mon = _get_async_monitor()
        # Should be the same object that mock_mon.async_monitor() returns.
        assert async_mon is mock_mon.async_monitor()
        assert isinstance(async_mon, AsyncMockMonitor)

    def test_returns_sync_to_async_wrapper_for_mock_monitor_subclass(self):
        """
        For a MockMonitor subclass, async_monitor() returns a _sync_to_async_monitor
        wrapper (so that overridden sync methods are called through the async interface).
        """

        class _SubMon(MockMonitor):
            pass

        SETTINGS.monitor = _SubMon(_SimpleMocks())
        assert _is_sync_to_async_wrapper(_get_async_monitor())

    def test_returns_passed_async_mock_monitor_when_wrapped(self):
        """When an AsyncMockMonitor is passed to set_mocks it is wrapped in a MockMonitor,
        but _get_async_monitor() unwraps it and returns the original instance."""
        async_mon = AsyncMockMonitor(_SimpleMocks())
        set_mocks(_SimpleMocks(), monitor=async_mon)
        assert _get_async_monitor() is async_mon

    def test_returns_sync_to_async_wrapper_for_arbitrary_monitor(self):
        """An arbitrary object without async_monitor gets wrapped in _sync_to_async_monitor."""

        class _ArbitraryMonitor:
            pass

        SETTINGS.monitor = _ArbitraryMonitor()
        assert _is_sync_to_async_wrapper(_get_async_monitor())

    @pytest.mark.asyncio
    async def test_async_register_package_on_mock_monitor(self):
        """Calling RegisterPackage through _get_async_monitor() works asynchronously."""
        set_mocks(_SimpleMocks())
        async_mon = _get_async_monitor()
        assert async_mon is not None

        response = await async_mon.RegisterPackage(  # type: ignore[attr-defined]
            resource_pb2.RegisterPackageRequest(name="mypkg", version="1.2.3")
        )
        assert response.ref == "mock-uuid"

    @pytest.mark.asyncio
    async def test_async_register_package_on_async_mock_monitor_passed_directly(self):
        """AsyncMockMonitor passed to set_mocks returns the right ref asynchronously."""
        async_mon = AsyncMockMonitor(_SimpleMocks())
        set_mocks(_SimpleMocks(), monitor=async_mon)

        result_mon = _get_async_monitor()
        assert result_mon is async_mon

        response = await result_mon.RegisterPackage(  # type: ignore[attr-defined]
            resource_pb2.RegisterPackageRequest(name="pkg", version="0.1.0")
        )
        assert response.ref == "mock-uuid"
