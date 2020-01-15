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
from typing import Any, Awaitable
import grpc

from ..output import Inputs
from ..invoke import InvokeOptions
from .. import log
from .settings import get_monitor
from ..runtime.proto import provider_pb2
from . import rpc
from .rpc_manager import RPC_MANAGER

# If we are not running on Python 3.7 or later, we need to swap the Python implementation of Task in for the C
# implementation in order to support synchronous invokes.
if sys.version_info[0] == 3 and sys.version_info[1] < 7:
    asyncio.Task = asyncio.tasks._PyTask
    asyncio.tasks.Task = asyncio.tasks._PyTask

    def enter_task(loop, task):
        task.__class__._current_tasks[loop] = task

    def leave_task(loop, task):
        task.__class__._current_tasks.pop(loop)

    _enter_task = enter_task
    _leave_task = leave_task
else:
    _enter_task = asyncio.tasks._enter_task # type: ignore
    _leave_task = asyncio.tasks._leave_task # type: ignore


def _sync_await(awaitable: Awaitable[Any]) -> Any:
    """
    _sync_await waits for the given future to complete by effectively yielding the current task and pumping the event
    loop.
    """

    # Fetch the current event loop and ensure a future.
    loop = asyncio.get_event_loop()
    fut = asyncio.ensure_future(awaitable)

    # If the loop is not running, we can just use run_until_complete. Without this, we would need to duplicate a fair
    # amount of bookkeeping logic around loop startup and shutdown.
    if not loop.is_running():
        return loop.run_until_complete(fut)

    # If we are executing inside a task, pretend we've returned from its current callback--effectively yielding to
    # the event loop--by calling _leave_task.
    task = asyncio.Task.current_task(loop)
    if task is not None:
        _leave_task(loop, task)

    # Pump the event loop until the future is complete. This is the kernel of BaseEventLoop.run_forever, and may not
    # work with alternative event loop implementations.
    #
    # In order to make this reentrant with respect to _run_once, we keep track of the number of event handles on the
    # ready list and ensure that there are exactly that many handles on the list once we are finished.
    #
    # See https://github.com/python/cpython/blob/3.6/Lib/asyncio/base_events.py#L1428-L1452 for the details of the
    # _run_once kernel with which we need to cooperate.
    ntodo = len(loop._ready) # type: ignore
    while not fut.done() and not fut.cancelled():
        loop._run_once() # type: ignore
        if loop._stopping: # type: ignore
            break
    # If we drained the ready list past what a calling _run_once would have expected, fix things up by pushing
    # cancelled handles onto the list.
    while len(loop._ready) < ntodo: # type: ignore
        handle = asyncio.Handle(lambda: None, [], loop)
        handle._cancelled = True
        loop._ready.append(handle) # type: ignore

    # If we were executing inside a task, restore its context and continue on.
    if task is not None:
        _enter_task(loop, task)

    # Return the result of the future.
    return fut.result()

class InvokeResult:
    """
    InvokeResult is a helper type that wraps a prompt value in an Awaitable.
    """
    def __init__(self, value):
        self.value = value

    # pylint: disable=using-constant-test
    def __await__(self):
        # We need __await__ to be an iterator, but we only want it to return one value. As such, we use
        # `if False: yield` to construct this.
        if False:
            yield self.value
        return self.value

    __iter__ = __await__

def invoke(tok: str, props: Inputs, opts: InvokeOptions = None) -> InvokeResult:
    """
    invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
    can be a bag of computed values (Ts or Awaitable[T]s), and the result is a Awaitable[Any] that
    resolves when the invoke finishes.
    """
    log.debug(f"Invoking function: tok={tok}")
    if opts is None:
        opts = InvokeOptions()

    async def do_invoke():
        # If a parent was provided, but no provider was provided, use the parent's provider if one was specified.
        if opts.parent is not None and opts.provider is None:
            opts.provider = opts.parent.get_provider(tok)

        # Construct a provider reference from the given provider, if one was provided to us.
        provider_ref = None
        if opts.provider is not None:
            provider_urn = await opts.provider.urn.future()
            provider_id = (await opts.provider.id.future()) or rpc.UNKNOWN
            provider_ref = f"{provider_urn}::{provider_id}"
            log.debug(f"Invoke using provider {provider_ref}")

        monitor = get_monitor()
        inputs = await rpc.serialize_properties(props, {})
        version = opts.version or ""
        log.debug(f"Invoking function prepared: tok={tok}")
        req = provider_pb2.InvokeRequest(tok=tok, args=inputs, provider=provider_ref, version=version)

        def do_invoke():
            try:
                return monitor.Invoke(req)
            except grpc.RpcError as exn:
                # gRPC-python gets creative with their exceptions. grpc.RpcError as a type is useless;
                # the usefullness come from the fact that it is polymorphically also a grpc.Call and thus has
                # the .code() member. Pylint doesn't know this because it's not known statically.
                #
                # Neither pylint nor I are the only ones who find this confusing:
                # https://github.com/grpc/grpc/issues/10885#issuecomment-302581315
                # pylint: disable=no-member
                if exn.code() == grpc.StatusCode.UNAVAILABLE:
                    sys.exit(0)

                details = exn.details()
            raise Exception(details)

        resp = await asyncio.get_event_loop().run_in_executor(None, do_invoke)

        log.debug(f"Invoking function completed successfully: tok={tok}")
        # If the invoke failed, raise an error.
        if resp.failures:
            raise Exception(f"invoke of {tok} failed: {resp.failures[0].reason} ({resp.failures[0].property})")

        # Otherwise, return the output properties.
        ret_obj = getattr(resp, 'return')
        if ret_obj:
            return rpc.deserialize_properties(ret_obj)
        return {}

    async def do_rpc():
        resp, exn = await RPC_MANAGER.do_rpc("invoke", do_invoke)()
        if exn is not None:
            raise exn
        return resp

    return InvokeResult(_sync_await(asyncio.ensure_future(do_rpc())))
