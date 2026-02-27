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

import asyncio
import inspect
from typing import TYPE_CHECKING, Optional, Protocol, cast, runtime_checkable
from collections.abc import Callable

import grpc

from ._grpc_settings import _GRPC_CHANNEL_OPTIONS
from .proto.resource_pb2_grpc import ResourceMonitorStub
from .sync_await import _sync_await

if TYPE_CHECKING:
    from .proto.resource_pb2_grpc import ResourceMonitorAsyncStub
else:
    ResourceMonitorAsyncStub = object  # make cast work at runtime


@runtime_checkable
class _ProvidesAsyncMonitor(Protocol):
    """
    A protocol for monitors that and provide an async monitor interface, either directly or via a wrapper.
    """

    def async_monitor(self) -> ResourceMonitorAsyncStub:
        """
        Returns the async monitor interface provided by this monitor, if any.
        This may be the monitor itself, an underlying async monitor if this is a sync wrapper,
        or an async wrapper around this monitor if this is a sync monitor without its own async
        interface.
        """
        ...


def _get_async_monitor_method(
    monitor,
) -> Optional[Callable[[], ResourceMonitorAsyncStub]]:
    """
    Returns the monitor's async_monitor method if it exists and is callable, otherwise returns None.
    This is used to check if the monitor provides an async_monitor method without triggering
    __getattr__/dynamic resolution, which a user-specified test mock monitor may do.
    """
    name = _ProvidesAsyncMonitor.async_monitor.__name__

    try:
        raw = inspect.getattr_static(monitor, name)
    except AttributeError:
        return None

    # raw may be function/descriptor/property/etc. Check callability of the descriptor object itself:
    if not callable(raw):
        return None

    # Now bind it the normal way (so you call the bound method):
    bound = getattr(monitor, name)
    if not callable(bound):
        return None

    return cast(Callable[[], ResourceMonitorAsyncStub], bound)


def _async_to_sync_monitor(monitor: ResourceMonitorAsyncStub) -> ResourceMonitorStub:
    """
    Factory for wrapping an async monitor in a sync interface.
    The returned monitor will implement the ResourceMonitorStub interface by synchronously waiting on the async
    monitor's methods, and also provide access to the async monitor via the _ProvidesAsyncMonitor protocol.
    """

    class AsyncToSyncMonitorProxy:
        def __init__(self, monitor: ResourceMonitorAsyncStub):
            assert isinstance(self, _ProvidesAsyncMonitor)
            self._monitor = monitor

        def async_monitor(self) -> ResourceMonitorAsyncStub:
            return self._monitor

        def __getattr__(self, name):
            attr = getattr(self._monitor, name)
            if not callable(attr):
                return attr

            def sync_call(*args, **kwargs):
                return _sync_await(attr(*args, **kwargs))  # type: ignore

            return sync_call

    proxy = AsyncToSyncMonitorProxy(monitor)
    return cast(ResourceMonitorStub, proxy)


def _sync_to_async_monitor(monitor: ResourceMonitorStub) -> ResourceMonitorAsyncStub:
    """
    Factory for wrapping a sync monitor in an async interface.
    The returned monitor will implement the ResourceMonitorAsyncStub interface by asynchronously
    executing the sync monitor's methods in a separate thread, and also provide access to the
    async monitor (itself) via the _ProvidesAsyncMonitor protocol.
    """

    class SyncToAsyncMonitorProxy:
        def __init__(self, monitor: ResourceMonitorStub):
            assert isinstance(self, _ProvidesAsyncMonitor)
            self._monitor = monitor

        def async_monitor(self) -> ResourceMonitorAsyncStub:
            return cast(ResourceMonitorAsyncStub, self)

        def __getattr__(self, name):
            attr = getattr(self._monitor, name)
            if not callable(attr):
                return attr

            async def async_call(*args, **kwargs):
                return await asyncio.to_thread(attr, *args, **kwargs)

            return async_call

    proxy = SyncToAsyncMonitorProxy(monitor)
    return cast(ResourceMonitorAsyncStub, proxy)


def _lazy_async_monitor(
    target: str, factory: Callable[[str], ResourceMonitorAsyncStub]
) -> ResourceMonitorAsyncStub:
    """
    Factory for creating a lazy async monitor from a factory that creates async monitors.
    This is used to lazily create the async monitor for the gRPC case, since it needs to be created
    in the context of a running event loop.
    """

    _monitor: Optional[ResourceMonitorAsyncStub] = None

    def _get_monitor() -> ResourceMonitorAsyncStub:
        nonlocal _monitor
        if _monitor is None:
            _monitor = factory(target)
        return _monitor

    class LazyAsyncMonitorProxy:
        def async_monitor(self) -> ResourceMonitorAsyncStub:
            return _get_monitor()

        def __getattr__(self, name: str):
            return getattr(_get_monitor(), name)

    return cast(ResourceMonitorAsyncStub, LazyAsyncMonitorProxy())


def _grpc_monitor(target: str) -> ResourceMonitorStub:
    """
    Factory for creating a gRPC monitor.
    """

    # Lazily create an async gRPC monitor using grpc.aio upon first attr access to ensure it binds
    # to the correct running event loop, otherwise it can result in "Future attached to a
    # different loop" errors when the monitor is first used.
    # Then wrap it in a proxy that provides a synchronous interface, with a way to access the
    # underlying async monitor for use in async contexts.
    #
    # The result of this function is what will be stored in SETTINGS.monitor and return from get_monitor,
    # so it needs to be a sync monitor that existing callers expect.
    # Inside this library _get_async_monitor should be used over get_monitor, which returns the underlying
    # async monitor.

    def factory(target: str):
        # The actual class is ResourceMonitorStub, but we know we're using an grpc.aio channel,
        # so cast it to ResourceMonitorAsyncStub so we get the correct type hints.
        return cast(
            ResourceMonitorAsyncStub,
            ResourceMonitorStub(
                grpc.aio.insecure_channel(target, options=_GRPC_CHANNEL_OPTIONS)
            ),
        )

    monitor = _lazy_async_monitor(target, factory)
    return _async_to_sync_monitor(monitor)
