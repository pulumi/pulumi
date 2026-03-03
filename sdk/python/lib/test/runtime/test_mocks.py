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
from pulumi.runtime.settings import ROOT, SETTINGS, _get_async_monitor, get_monitor
from pulumi.runtime.sync_await import _ensure_event_loop


class _SimpleMocks(Mocks):
    def call(self, args):
        return {}, None

    def new_resource(self, args):
        return args.name + "_id", {}


# SyncToAsyncMonitorProxy is defined *inside* _sync_to_async_monitor, so each call
# produces a distinct class object.  All of them share the same __qualname__, though,
# so we capture it once from a known-good reference instance.
_SYNC_TO_ASYNC_PROXY_QUALNAME: str = type(
    _sync_to_async_monitor(cast(object, object()))  # type: ignore[arg-type]
).__qualname__


def _is_sync_to_async_wrapper(obj) -> bool:
    """Returns True when obj is a _sync_to_async_monitor proxy."""
    return type(obj).__qualname__ == _SYNC_TO_ASYNC_PROXY_QUALNAME


@pytest.fixture(autouse=True)
def _reset_settings():
    """Isolate every test: restore the monitor and root-resource context vars."""
    _ensure_event_loop()
    old_monitor = SETTINGS.monitor
    old_root = ROOT.get()
    yield
    SETTINGS.monitor = old_monitor
    ROOT.set(old_root)


# Test arbitrary (non-MockMonitor / non-AsyncMockMonitor) object.
class TestArbitraryMonitor:
    class _ArbitraryMonitor:
        """A plain monitor object that doesn't subclass MockMonitor or AsyncMockMonitor."""

    def setup_method(self):
        self.arb = self._ArbitraryMonitor()
        set_mocks(_SimpleMocks(), monitor=self.arb)  # type: ignore[arg-type]

    def test_get_monitor_returns_the_object(self):
        assert get_monitor() is self.arb

    def test_get_async_monitor_returns_sync_to_async_wrapper(self):
        async_mon = _get_async_monitor()
        assert _is_sync_to_async_wrapper(async_mon)

    def test_async_wrapper_wraps_the_arbitrary_object(self):
        # The proxy's __getattr__ delegates to the wrapped sync object.
        # We can confirm by checking that the proxy's async_monitor() is itself
        # (self-referential, distinguishing it from the raw arbitrary object).
        async_mon = _get_async_monitor()
        assert async_mon is not self.arb


# Test MockMonitor subclass with an overridden method.
class TestMockMonitorSubclass:
    class _SubMockMonitor(MockMonitor):
        def __init__(self, mocks):
            super().__init__(mocks)
            self.feature_calls = 0

        def SupportsFeature(self, request):
            self.feature_calls += 1
            return type("R", (), {"hasSupport": True})()

    def setup_method(self):
        self.sub_mon = self._SubMockMonitor(_SimpleMocks())
        set_mocks(_SimpleMocks(), monitor=self.sub_mon)

    def test_get_monitor_returns_same_instance(self):
        assert get_monitor() is self.sub_mon

    def test_get_async_monitor_returns_sync_to_async_wrapper(self):
        async_mon = _get_async_monitor()
        # For a subclass, MockMonitor.async_monitor() wraps self in _sync_to_async_monitor.
        assert _is_sync_to_async_wrapper(async_mon)

    def test_async_wrapper_is_not_the_async_mock_monitor(self):
        async_mon = _get_async_monitor()
        assert not isinstance(async_mon, AsyncMockMonitor)

    @pytest.mark.asyncio
    async def test_overridden_method_is_called_through_async_wrapper(self):
        """Awaiting a method through the async wrapper invokes the subclass override."""
        async_mon = _get_async_monitor()
        request = type("Req", (), {"id": "test"})()
        await async_mon.SupportsFeature(request)  # type: ignore[attr-defined]
        assert self.sub_mon.feature_calls == 1


# Test plain MockMonitor (not a subclass).
class TestPlainMockMonitor:
    def setup_method(self):
        self.mock_mon = MockMonitor(_SimpleMocks())
        set_mocks(_SimpleMocks(), monitor=self.mock_mon)

    def test_get_monitor_returns_same_instance(self):
        assert get_monitor() is self.mock_mon

    def test_get_async_monitor_returns_async_mock_monitor(self):
        async_mon = _get_async_monitor()
        assert isinstance(async_mon, AsyncMockMonitor)

    def test_get_async_monitor_matches_mock_monitor_async_monitor(self):
        # mock_mon.async_monitor() should return the same object as _get_async_monitor().
        assert self.mock_mon.async_monitor() is _get_async_monitor()

    def test_async_monitor_is_not_the_mock_monitor_itself(self):
        assert _get_async_monitor() is not self.mock_mon


# Test AsyncMockMonitor passed directly.
class TestAsyncMockMonitorPassedDirectly:
    def setup_method(self):
        self.async_mon = AsyncMockMonitor(_SimpleMocks())
        set_mocks(_SimpleMocks(), monitor=self.async_mon)

    def test_get_monitor_returns_mock_monitor_wrapper(self):
        mon = get_monitor()
        assert isinstance(mon, MockMonitor)
        assert mon is not self.async_mon

    def test_mock_monitor_async_monitor_returns_the_passed_instance(self):
        mon = get_monitor()
        assert mon.async_monitor() is self.async_mon  # type: ignore[attr-defined]

    def test_get_async_monitor_returns_the_passed_instance(self):
        assert _get_async_monitor() is self.async_mon


# Test default monitor (no monitor passed).
class TestDefaultMonitor:
    def setup_method(self):
        set_mocks(_SimpleMocks())

    def test_get_monitor_returns_mock_monitor(self):
        assert isinstance(get_monitor(), MockMonitor)

    def test_get_async_monitor_returns_async_mock_monitor(self):
        assert isinstance(_get_async_monitor(), AsyncMockMonitor)

    def test_mock_monitor_async_monitor_matches_get_async_monitor(self):
        mon = get_monitor()
        assert mon.async_monitor() is _get_async_monitor()  # type: ignore[attr-defined]
