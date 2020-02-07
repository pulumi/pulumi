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
import sys
import traceback
from typing import Callable, Awaitable, Tuple, Any, Optional, List
from .. import log


class RPCManager:
    """
    RPCManager is responsible for keeping track of RPCs that are dispatched
    throughout the course of a Pulumi program. It records exceptions that occur
    when performing RPC calls and keeps track of whether or not there are any
    outstanding RPCs.
    """

    rpcs: List[Awaitable]
    """
    The active RPCs.
    """

    unhandled_exception: Optional[Exception]
    """
    The first unhandled exception encountered during an RPC, if any occurs.
    """

    exception_traceback: Optional[Any]
    """
    The traceback associated with unhandled_exception, if any.
    """

    def __init__(self):
        self.rpcs = []
        self.unhandled_exception = None
        self.exception_traceback = None

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

            rpc = asyncio.ensure_future(rpc_function(*args, **kwargs))
            self.rpcs.append(rpc)
            try:
                result = await rpc
                exception = None
            except Exception as exn:
                log.debug(f"RPC failed with exception:")
                log.debug(traceback.format_exc())
                if self.unhandled_exception is None:
                    self.unhandled_exception = exn
                    self.exception_traceback = sys.exc_info()[2]
                result = None
                exception = exn

            return result, exception

        return rpc_wrapper


RPC_MANAGER: RPCManager = RPCManager()
"""
Singleton RPC manager responsible for coordinating RPC calls to the engine.
"""
