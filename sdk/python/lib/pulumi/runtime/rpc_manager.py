# Copyright 2016-2018, Pulumi Corporation.
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
import traceback
from typing import Callable, Awaitable, Tuple, Any
from .. import log


class RPCManager:
    """
    RPCManager is responsible for keeping track of RPCs that are dispatched
    throughout the course of a Pulumi program. It records exceptions that occur
    when performing RPC calls and keeps track of whether or not there are any
    outstanding RPCs.
    """

    zero_cond: asyncio.Condition
    """
    zero_cond is notified whenever the number of active RPCs transitions from
    one to zero.
    """

    count: int
    """
    The number of active RPCs.
    """

    exception_future: asyncio.Future
    """
    Future that is resolved whenever an unhandled exception occurs.
    """

    def __init__(self):
        self.zero_cond = asyncio.Condition()
        self.count = 0
        self.exception_future = asyncio.Future()

    def do_rpc(self, name: str, rpc_function: Callable[..., Awaitable[Tuple[Any, Exception]]]) -> Callable[..., Awaitable[Tuple[Any, Exception]]]:
        """
        Wraps a given RPC function by producing an awaitable function suitable to be run in the asyncio
        event loop. The wrapped function catches all unhandled exceptions and reports them to the exception
        future, which consumers can await upon to listen for unhandled exceptions.

        The wrapped function also keeps track of the number of outstanding RPCs to synchronize during
        shutdown.
        :param name: The name of this RPC, to be used for logging
        :param rpc_function: The function implementing the RPC
        :return: An awaitable function implementing the RPC
        """
        async def rpc_wrapper(*args, **kwargs):
            log.debug(f"beginning rpc {name}")
            async with self.zero_cond:
                self.count += 1
                log.debug(f"recorded new RPC, {self.count} RPCs outstanding")

            try:
                result = await rpc_function(*args, **kwargs)
                exception = None
            except Exception as exn:
                log.debug(f"RPC failed with exception:")
                log.debug(traceback.format_exc())
                if not self.exception_future.done():
                    self.exception_future.set_exception(exn)
                result = None
                exception = exn

            async with self.zero_cond:
                self.count -= 1
                if self.count == 0:
                    log.debug("All RPC completed, signalling completion")
                    if not self.exception_future.done():
                        self.exception_future.set_result(None)
                    self.zero_cond.notify_all()
                log.debug(f"recorded RPC completion, {self.count} RPCs outstanding")

            return result, exception

        return rpc_wrapper

    async def wait_for_outstanding_rpcs(self) -> None:
        """
        Blocks the calling task until all outstanding RPCs have completed. Returns immediately if
        no RPCs have been initiated.
        """
        async with self.zero_cond:
            while self.count != 0:
                await self.zero_cond.wait()

    def unhandled_exeception(self) -> asyncio.Future:
        """
        Returns a Future that is resolved abnormally whenever an RPC fails due to an unhandled exception.
        """
        return self.exception_future


RPC_MANAGER: RPCManager = RPCManager()
"""
Singleton RPC manager responsible for coordinating RPC calls to the engine.
"""
