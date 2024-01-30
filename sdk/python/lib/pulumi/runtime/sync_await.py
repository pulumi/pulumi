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
from typing import Awaitable, TypeVar

T = TypeVar("T")


def _sync_await(awaitable: Awaitable[T]) -> T:
    """
    _sync_await waits for the given future to complete by effectively yielding the current task and pumping the event
    loop.
    """

    # Fetch the current event loop and ensure a future.
    loop = _ensure_event_loop()
    fut = asyncio.ensure_future(awaitable)

    # If the loop is not running, we can just use run_until_complete. Without this, we would need to duplicate a fair
    # amount of bookkeeping logic around loop startup and shutdown.
    if not loop.is_running():
        return loop.run_until_complete(fut)

    # If we are executing inside a task, pretend we've returned from its current callback--effectively yielding to
    # the event loop--by calling _leave_task.
    task = asyncio.current_task(loop)
    if task is not None:
        asyncio.tasks._leave_task(loop, task)

    # Pump the event loop until the future is complete. This is the kernel of BaseEventLoop.run_forever, and may not
    # work with alternative event loop implementations.
    #
    # In order to make this reentrant with respect to _run_once, we keep track of the number of event handles on the
    # ready list and ensure that there are exactly that many handles on the list once we are finished.
    #
    # See https://github.com/python/cpython/blob/3.6/Lib/asyncio/base_events.py#L1428-L1452 for the details of the
    # _run_once kernel with which we need to cooperate.
    ntodo = len(loop._ready)  # type: ignore
    while not fut.done() and not fut.cancelled():
        loop._run_once()  # type: ignore
        if loop._stopping:  # type: ignore
            break
    # If we drained the ready list past what a calling _run_once would have expected, fix things up by pushing
    # cancelled handles onto the list.
    while len(loop._ready) < ntodo:  # type: ignore
        handle = asyncio.Handle(lambda: None, [], loop)
        handle._cancelled = True
        loop._ready.append(handle)  # type: ignore

    # If we were executing inside a task, restore its context and continue on.
    if task is not None:
        asyncio.tasks._enter_task(loop, task)

    # Return the result of the future.
    return fut.result()


def _ensure_event_loop():
    """Ensures an asyncio event loop exists for the current thread."""
    loop = None
    try:
        loop = asyncio.get_event_loop()
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
    return loop
