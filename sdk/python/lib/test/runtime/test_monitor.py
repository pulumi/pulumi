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

from typing import TYPE_CHECKING, cast

import pytest

from unittest.mock import MagicMock, patch

from pulumi.runtime._grpc_settings import _GRPC_CHANNEL_OPTIONS
from pulumi.runtime._monitor import (
    _ProvidesAsyncMonitor,
    _async_to_sync_monitor,
    _get_async_monitor_method,
    _grpc_monitor,
    _lazy_async_monitor,
    _sync_to_async_monitor,
)

if TYPE_CHECKING:
    from pulumi.runtime.proto.resource_pb2_grpc import ResourceMonitorAsyncStub
else:
    ResourceMonitorAsyncStub = object  # make cast work at runtime


class TestGetAsyncMonitorMethod:
    def test_returns_none_when_no_async_monitor_attribute(self):
        """Monitor with no async_monitor attribute returns None."""

        class PlainMonitor:
            pass

        assert _get_async_monitor_method(PlainMonitor()) is None

    def test_returns_bound_method_when_async_monitor_is_regular_method(self):
        """Monitor with a normal async_monitor method returns the bound method."""
        sentinel = object()

        class MonitorWithMethod:
            def async_monitor(self):
                return sentinel

        monitor = MonitorWithMethod()
        result = _get_async_monitor_method(monitor)
        assert result is not None
        assert callable(result)
        assert result() is sentinel

    def test_returns_none_when_async_monitor_is_non_callable_attribute(self):
        """Monitor where async_monitor is a plain (non-callable) attribute returns None."""

        class MonitorWithNonCallable:
            async_monitor = 42

        assert _get_async_monitor_method(MonitorWithNonCallable()) is None

    def test_does_not_trigger_getattr_for_dynamic_monitor(self):
        """
        Monitors that use __getattr__ for dynamic dispatch (e.g. test mocks) must NOT
        have _get_async_monitor_method accidentally trigger that dynamic lookup and
        return a truthy result when async_monitor is not actually defined.
        """
        getattr_calls = []

        class DynamicMonitor:
            def __getattr__(self, name):
                getattr_calls.append(name)
                # A test mock might return a callable for any attribute lookup —
                # this should NOT cause _get_async_monitor_method to return non-None
                # unless async_monitor is explicitly defined on the class/instance.
                return lambda: None

        result = _get_async_monitor_method(DynamicMonitor())
        assert result is None
        # Confirm __getattr__ was not consulted for the static inspection phase
        assert "async_monitor" not in getattr_calls

    def test_returns_none_when_async_monitor_is_property_returning_non_callable(self):
        """Property that returns a non-callable value causes the bound check to return None."""

        class MonitorWithProperty:
            @property
            def async_monitor(self):
                return 99  # not callable

        # inspect.getattr_static sees the property object itself (which is callable),
        # but getattr returns the property's value (99), which is not callable.
        assert _get_async_monitor_method(MonitorWithProperty()) is None

    def test_returns_none_when_async_monitor_is_property(self):
        """
        A @property descriptor is not callable itself, so the static-inspection callable
        check returns None even if the property getter returns a callable.
        """
        sentinel = object()

        class MonitorWithCallableProperty:
            @property
            def async_monitor(self):
                return lambda: sentinel

        # The raw descriptor (property object) is not callable, so None is returned.
        assert _get_async_monitor_method(MonitorWithCallableProperty()) is None

    def test_returns_none_when_callable_descriptor_get_returns_non_callable(self):
        """
        A descriptor whose object is callable (has __call__) but whose __get__ returns
        a non-callable value hits the second callable check and returns None.
        """

        class CallableDescriptorReturningInt:
            """Callable as an object, but __get__ yields a plain int."""

            def __call__(self):
                pass

            def __get__(self, obj, objtype=None):
                return 99  # not callable

        class MonitorWithCallableDescriptor:
            async_monitor = CallableDescriptorReturningInt()

        # inspect.getattr_static -> descriptor (callable) passes first check;
        # getattr -> 99 (not callable) hits the second `return None`.
        assert _get_async_monitor_method(MonitorWithCallableDescriptor()) is None

    def test_returned_callable_is_bound_to_instance(self):
        """The returned callable is the bound method of the specific instance."""

        class MonitorWithMethod:
            def __init__(self, value):
                self.value = value

            def async_monitor(self):
                return self.value

        m1 = MonitorWithMethod("first")
        m2 = MonitorWithMethod("second")
        assert _get_async_monitor_method(m1)() == "first"
        assert _get_async_monitor_method(m2)() == "second"

    def test_returns_none_for_classmethod(self):
        """
        A classmethod descriptor object is not callable itself in Python 3.11, so
        the static-inspection callable check returns None.
        """

        class MonitorWithClassMethod:
            @classmethod
            def async_monitor(cls):
                return cls

        assert _get_async_monitor_method(MonitorWithClassMethod()) is None

    def test_works_with_staticmethod(self):
        """async_monitor defined as a staticmethod is callable and returned."""
        sentinel = object()

        class MonitorWithStaticMethod:
            @staticmethod
            def async_monitor():
                return sentinel

        result = _get_async_monitor_method(MonitorWithStaticMethod())
        assert result is not None
        assert result() is sentinel


class _AsyncMonitorStub:
    """Minimal async-monitor stand-in with a mix of callable and non-callable members."""

    non_callable_attr = "hello"

    async def get_value(self, x: int) -> int:
        return x * 2

    async def raises(self) -> None:
        raise ValueError("boom")


def _as_async_stub(obj) -> ResourceMonitorAsyncStub:
    return cast(ResourceMonitorAsyncStub, obj)


class TestAsyncToSyncMonitor:
    def _make_proxy(self):
        return _async_to_sync_monitor(_as_async_stub(_AsyncMonitorStub()))

    def test_proxy_satisfies_provides_async_monitor_protocol(self):
        """The returned proxy is recognised as a _ProvidesAsyncMonitor."""
        proxy = self._make_proxy()
        assert isinstance(proxy, _ProvidesAsyncMonitor)

    def test_async_monitor_returns_wrapped_async_monitor(self):
        """proxy.async_monitor() returns the original async monitor object."""
        inner = _AsyncMonitorStub()
        proxy = _async_to_sync_monitor(_as_async_stub(inner))
        assert proxy.async_monitor() is inner  # type: ignore[attr-defined]

    def test_async_monitor_method_visible_to_get_async_monitor_method(self):
        """
        _get_async_monitor_method should see async_monitor on the proxy and return a
        bound callable, confirming the two helpers compose correctly.
        """
        inner = _AsyncMonitorStub()
        proxy = _async_to_sync_monitor(_as_async_stub(inner))
        bound = _get_async_monitor_method(proxy)
        assert bound is not None
        assert bound() is inner

    def test_sync_call_returns_result_of_awaited_coroutine(self):
        """Calling a proxied async method synchronously returns its awaited result."""
        proxy = self._make_proxy()
        result = proxy.get_value(21)  # type: ignore[attr-defined]
        assert result == 42

    def test_sync_call_passes_args_and_kwargs_to_async_method(self):
        """Positional and keyword arguments are forwarded to the underlying async method."""

        class ArgsMonitor:
            async def echo(self, a, b, *, flag=False):
                return (a, b, flag)

        proxy = _async_to_sync_monitor(_as_async_stub(ArgsMonitor()))
        result = proxy.echo(1, 2, flag=True)  # type: ignore[attr-defined]
        assert result == (1, 2, True)

    def test_sync_call_propagates_exception_from_coroutine(self):
        """Exceptions raised inside the async method surface as-is to the sync caller."""
        proxy = self._make_proxy()
        with pytest.raises(ValueError, match="boom"):
            proxy.raises()  # type: ignore[attr-defined]

    def test_each_attribute_access_returns_same_sync_wrapper_behaviour(self):
        """Repeated attribute lookups via __getattr__ each produce a working wrapper."""
        proxy = self._make_proxy()
        # Access the same method twice and verify both wrappers produce correct results.
        assert proxy.get_value(3) == 6  # type: ignore[attr-defined]
        assert proxy.get_value(5) == 10  # type: ignore[attr-defined]

    def test_non_callable_attribute_is_returned_directly(self):
        """Non-callable attributes on the async monitor are passed through unchanged."""
        proxy = self._make_proxy()
        assert proxy.non_callable_attr == "hello"  # type: ignore[attr-defined]

    def test_each_proxy_wraps_its_own_inner_monitor(self):
        """Two proxies created from different async monitors are independent."""
        inner_a = _AsyncMonitorStub()
        inner_b = _AsyncMonitorStub()
        proxy_a = _async_to_sync_monitor(_as_async_stub(inner_a))
        proxy_b = _async_to_sync_monitor(_as_async_stub(inner_b))
        assert proxy_a.async_monitor() is inner_a  # type: ignore[attr-defined]
        assert proxy_b.async_monitor() is inner_b  # type: ignore[attr-defined]
        assert proxy_a.async_monitor() is not proxy_b.async_monitor()  # type: ignore[attr-defined]


class _SyncMonitorStub:
    """Minimal sync-monitor stand-in with a mix of callable and non-callable members."""

    non_callable_attr = "world"

    def get_value(self, x: int) -> int:
        return x * 3

    def raises(self) -> None:
        raise RuntimeError("sync-boom")


class TestSyncToAsyncMonitor:
    def _make_proxy(self):
        return _sync_to_async_monitor(cast(object, _SyncMonitorStub()))  # type: ignore[arg-type]

    def test_proxy_satisfies_provides_async_monitor_protocol(self):
        """The returned proxy is recognised as a _ProvidesAsyncMonitor."""
        proxy = self._make_proxy()
        assert isinstance(proxy, _ProvidesAsyncMonitor)

    def test_async_monitor_returns_self(self):
        """proxy.async_monitor() returns the proxy itself, not the inner sync monitor."""
        inner = _SyncMonitorStub()
        proxy = _sync_to_async_monitor(cast(object, inner))  # type: ignore[arg-type]
        result = proxy.async_monitor()  # type: ignore[attr-defined]
        assert result is proxy
        assert result is not inner

    def test_async_monitor_method_visible_to_get_async_monitor_method(self):
        """
        _get_async_monitor_method should see async_monitor on the proxy and return a
        bound callable that returns the proxy itself.
        """
        proxy = self._make_proxy()
        bound = _get_async_monitor_method(proxy)
        assert bound is not None
        assert bound() is proxy

    @pytest.mark.asyncio
    async def test_async_call_returns_result_of_sync_method(self):
        """Awaiting a proxied sync method returns its result."""
        proxy = self._make_proxy()
        result = await proxy.get_value(14)  # type: ignore[attr-defined]
        assert result == 42

    @pytest.mark.asyncio
    async def test_async_call_passes_args_and_kwargs_to_sync_method(self):
        """Positional and keyword arguments are forwarded to the underlying sync method."""

        class ArgsMonitor:
            def echo(self, a, b, *, flag=False):
                return (a, b, flag)

        proxy = _sync_to_async_monitor(cast(object, ArgsMonitor()))  # type: ignore[arg-type]
        result = await proxy.echo(1, 2, flag=True)  # type: ignore[attr-defined]
        assert result == (1, 2, True)

    @pytest.mark.asyncio
    async def test_async_call_propagates_exception_from_sync_method(self):
        """Exceptions raised inside the sync method surface when the coroutine is awaited."""
        proxy = self._make_proxy()
        with pytest.raises(RuntimeError, match="sync-boom"):
            await proxy.raises()  # type: ignore[attr-defined]

    @pytest.mark.asyncio
    async def test_each_attribute_access_returns_awaitable(self):
        """Repeated attribute lookups each produce a working async wrapper."""
        proxy = self._make_proxy()
        assert await proxy.get_value(4) == 12  # type: ignore[attr-defined]
        assert await proxy.get_value(7) == 21  # type: ignore[attr-defined]

    def test_non_callable_attribute_is_returned_directly(self):
        """Non-callable attributes on the sync monitor are passed through unchanged."""
        proxy = self._make_proxy()
        assert proxy.non_callable_attr == "world"  # type: ignore[attr-defined]

    def test_each_proxy_wraps_its_own_inner_monitor(self):
        """Two proxies created from different sync monitors are independent."""
        inner_a = _SyncMonitorStub()
        inner_b = _SyncMonitorStub()
        proxy_a = _sync_to_async_monitor(cast(object, inner_a))  # type: ignore[arg-type]
        proxy_b = _sync_to_async_monitor(cast(object, inner_b))  # type: ignore[arg-type]
        assert proxy_a.async_monitor() is proxy_a  # type: ignore[attr-defined]
        assert proxy_b.async_monitor() is proxy_b  # type: ignore[attr-defined]
        assert proxy_a is not proxy_b


class _CountingFactory:
    """A factory that records calls and returns a fixed inner monitor."""

    def __init__(self, inner: ResourceMonitorAsyncStub):
        self.calls: list[str] = []
        self._inner = inner

    def __call__(self, target: str) -> ResourceMonitorAsyncStub:
        self.calls.append(target)
        return self._inner


class TestLazyAsyncMonitor:
    def _make_factory(self) -> tuple[_CountingFactory, ResourceMonitorAsyncStub]:
        inner = _as_async_stub(_AsyncMonitorStub())
        return _CountingFactory(inner), inner

    def test_factory_not_called_at_construction(self):
        """The factory is NOT invoked when the proxy is created."""
        factory, _ = self._make_factory()
        _lazy_async_monitor("target", factory)
        assert factory.calls == []

    def test_factory_called_on_first_getattr(self):
        """The factory is invoked on the first __getattr__ access."""
        factory, _ = self._make_factory()
        proxy = _lazy_async_monitor("target", factory)
        _ = proxy.get_value  # type: ignore[attr-defined]
        assert len(factory.calls) == 1

    def test_factory_called_with_target(self):
        """The factory receives the target string that was passed to _lazy_async_monitor."""
        factory, _ = self._make_factory()
        proxy = _lazy_async_monitor("my-target", factory)
        _ = proxy.get_value  # type: ignore[attr-defined]
        assert factory.calls == ["my-target"]

    def test_factory_called_only_once(self):
        """Multiple attribute accesses only trigger the factory once (memoised)."""
        factory, _ = self._make_factory()
        proxy = _lazy_async_monitor("target", factory)
        _ = proxy.get_value  # type: ignore[attr-defined]
        _ = proxy.get_value  # type: ignore[attr-defined]
        _ = proxy.raises  # type: ignore[attr-defined]
        assert len(factory.calls) == 1

    def test_async_monitor_triggers_factory(self):
        """Calling async_monitor() also triggers the factory."""
        factory, _ = self._make_factory()
        proxy = _lazy_async_monitor("target", factory)
        proxy.async_monitor()  # type: ignore[attr-defined]
        assert len(factory.calls) == 1

    def test_async_monitor_returns_inner_monitor(self):
        """async_monitor() returns the monitor produced by the factory."""
        factory, inner = self._make_factory()
        proxy = _lazy_async_monitor("target", factory)
        assert proxy.async_monitor() is inner  # type: ignore[attr-defined]

    def test_attributes_delegated_to_inner_monitor(self):
        """__getattr__ forwards attribute lookups to the underlying async monitor."""
        factory, inner = self._make_factory()
        proxy = _lazy_async_monitor("target", factory)
        # Bound method objects differ per-access, but the underlying function must be
        # identical, confirming the proxy delegates directly without any wrapping.
        assert proxy.get_value.__func__ is inner.get_value.__func__  # type: ignore[attr-defined]

    def test_non_callable_attribute_delegated_to_inner_monitor(self):
        """Non-callable attributes are also forwarded to the inner monitor."""
        factory, _ = self._make_factory()
        proxy = _lazy_async_monitor("target", factory)
        assert proxy.non_callable_attr == "hello"  # type: ignore[attr-defined]

    def test_proxy_satisfies_provides_async_monitor_protocol(self):
        """The returned proxy is recognised as a _ProvidesAsyncMonitor."""
        factory, _ = self._make_factory()
        proxy = _lazy_async_monitor("target", factory)
        assert isinstance(proxy, _ProvidesAsyncMonitor)

    def test_get_async_monitor_method_sees_async_monitor(self):
        """_get_async_monitor_method returns a bound callable for the proxy."""
        factory, inner = self._make_factory()
        proxy = _lazy_async_monitor("target", factory)
        bound = _get_async_monitor_method(proxy)
        assert bound is not None
        assert bound() is inner

    def test_each_proxy_has_independent_factory_and_inner_monitor(self):
        """Two proxies backed by different factories/targets are fully independent."""
        inner_a = _as_async_stub(_AsyncMonitorStub())
        inner_b = _as_async_stub(_AsyncMonitorStub())
        factory_a = _CountingFactory(inner_a)
        factory_b = _CountingFactory(inner_b)
        proxy_a = _lazy_async_monitor("target-a", factory_a)
        proxy_b = _lazy_async_monitor("target-b", factory_b)

        assert proxy_a.async_monitor() is inner_a  # type: ignore[attr-defined]
        assert proxy_b.async_monitor() is inner_b  # type: ignore[attr-defined]
        assert factory_a.calls == ["target-a"]
        assert factory_b.calls == ["target-b"]


_PATCH_CHANNEL = "grpc.aio.insecure_channel"


def _trigger_factory(proxy) -> None:
    """
    Force the lazy gRPC factory to run by drilling through the two proxy layers:
      AsyncToSyncProxy.async_monitor() -> LazyAsyncProxy
      LazyAsyncProxy.async_monitor()   -> calls _get_monitor() -> factory
    """
    lazy = proxy.async_monitor()  # type: ignore[attr-defined]
    lazy.async_monitor()  # type: ignore[attr-defined]


class TestGrpcMonitor:
    def test_returns_provides_async_monitor(self):
        """The returned proxy satisfies the _ProvidesAsyncMonitor protocol."""
        with patch(_PATCH_CHANNEL):
            proxy = _grpc_monitor("host:1234")
        assert isinstance(proxy, _ProvidesAsyncMonitor)

    def test_get_async_monitor_method_works(self):
        """_get_async_monitor_method sees async_monitor on the returned proxy."""
        with patch(_PATCH_CHANNEL):
            proxy = _grpc_monitor("host:1234")
        bound = _get_async_monitor_method(proxy)
        assert bound is not None
        assert callable(bound)

    def test_channel_not_created_at_construction(self):
        """grpc.aio.insecure_channel is NOT called when _grpc_monitor is called."""
        with patch(_PATCH_CHANNEL) as mock_channel:
            _grpc_monitor("host:1234")
            mock_channel.assert_not_called()

    def test_channel_created_on_first_access(self):
        """grpc.aio.insecure_channel is called on the first attribute access."""
        with patch(_PATCH_CHANNEL) as mock_channel:
            proxy = _grpc_monitor("host:1234")
            mock_channel.assert_not_called()
            _trigger_factory(proxy)
            mock_channel.assert_called_once()

    def test_channel_created_with_correct_target(self):
        """The target string is forwarded to grpc.aio.insecure_channel."""
        with patch(_PATCH_CHANNEL) as mock_channel:
            proxy = _grpc_monitor("my-host:9090")
            _trigger_factory(proxy)
        assert mock_channel.call_args.args[0] == "my-host:9090"

    def test_channel_created_with_correct_options(self):
        """The standard _GRPC_CHANNEL_OPTIONS are forwarded to grpc.aio.insecure_channel."""
        with patch(_PATCH_CHANNEL) as mock_channel:
            proxy = _grpc_monitor("host:1234")
            _trigger_factory(proxy)
        _, kwargs = mock_channel.call_args
        assert kwargs.get("options") == _GRPC_CHANNEL_OPTIONS

    def test_channel_created_only_once(self):
        """Multiple accesses only create the gRPC channel once."""
        with patch(_PATCH_CHANNEL) as mock_channel:
            proxy = _grpc_monitor("host:1234")
            _trigger_factory(proxy)
            _trigger_factory(proxy)
            _trigger_factory(proxy)
        mock_channel.assert_called_once()
