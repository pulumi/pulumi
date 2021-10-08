# Copyright 2016-2021, Pulumi Corporation.
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
from typing import Awaitable, Optional, List, Callable, Tuple, Any
from .rpc_manager import RPC_MANAGER


class TaskManager:
    """
    TaskManager is responsible for keeping track of async tasks that are dispatched
    throughout the course of a Pulumi program.
    """

    tasks: List[Awaitable]
    """
    The active tasks.
    """

    def __init__(self):
        self.clear()

    def create_task(self, coro: Awaitable, name: Optional[str] = None) -> asyncio.Future:
        """
        Wraps a given RPC function by producing an awaitable function suitable to be run in the asyncio
        event loop. The wrapped function catches all unhandled exceptions and reports them to the exception
        future, which consumers can await upon to listen for unhandled exceptions.

        The wrapped function also keeps track of the number of outstanding RPCs to synchronize during
        shutdown.
        :param coro: The coroutine
        :param name: The name of this task, to be used for logging
        :return: The task
        """
        fut = asyncio.ensure_future(coro)
        self.tasks.append(fut)
        return fut

    def do_rpc(self, name: str,
               rpc_function: Callable[..., Awaitable[Tuple[Any, Exception]]]
               ) -> asyncio.Future:
        return self.create_task(RPC_MANAGER.do_rpc(name, rpc_function)())

    def clear(self) -> None:
        """Clears any tracked state. For use in testing to ensure test isolation."""
        self.tasks = []


TASK_MANAGER: TaskManager = TaskManager()
"""
Singleton task manager responsible for tracking outstanding async tasks.
"""
