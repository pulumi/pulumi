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

"""
Support for automatic stack components.
"""
import asyncio
from typing import Callable, Any, Dict

from ..resource import ComponentResource
from .settings import get_project, get_stack, get_root_resource, set_root_resource
from .rpc_manager import RPC_MANAGER
from .. import log


async def run_in_stack(func: Callable):
    """
    Run the given function inside of a new stack resource.  This ensures that any stack export calls will end
    up as output properties on the resulting stack component in the checkpoint file.  This is meant for internal
    runtime use only and is used by the Python SDK entrypoint program.
    """
    try:
        Stack(func)

        # If an exception occurred when doing an RPC, this await will propegate the exception
        # to the main thread.
        await RPC_MANAGER.unhandled_exeception()
    finally:
        log.debug("Waiting for outstanding RPCs to complete")

        # Pump the event loop, giving all of the RPCs that we just queued up time to fully execute.
        # The asyncio scheduler does not expose a "yield" primitive, so this will have to do.
        #
        # Note that "asyncio.sleep(0)" is the blessed way to do this:
        # https://github.com/python/asyncio/issues/284#issuecomment-154180935
        await asyncio.sleep(0)

        # Wait for all outstanding RPCs to retire.
        await RPC_MANAGER.wait_for_outstanding_rpcs()

        # Asyncio event loops require that all outstanding tasks be completed by the time that the event
        # loop closes. If we're at this point and there are no outstanding RPCs, we should just cancel
        # all outstanding tasks.
        #
        # We will occasionally start tasks deliberately that we know will never complete. We must cancel
        # them before shutting down the event loop.
        log.debug("Canceling all outstanding tasks")
        for task in asyncio.Task.all_tasks():
            # Don't kill ourselves, that would be silly.
            if task == asyncio.Task.current_task():
                continue
            task.cancel()

        # Pump the event loop again. Task.cancel is delivered asynchronously to all running tasks
        # and each task needs to get scheduled in order to acknowledge the cancel and exit.
        await asyncio.sleep(0)

        # Once we get scheduled again, all tasks have exited and we're good to go.
        log.debug("run_in_stack completed")


class Stack(ComponentResource):
    """
    A synthetic stack component that automatically parents resources as the program runs.
    """

    outputs: Dict[str, Any]

    def __init__(self, func: Callable) -> None:
        # Ensure we don't already have a stack registered.
        if get_root_resource() is not None:
            raise Exception('Only one root Pulumi Stack may be active at once')

        # Now invoke the registration to begin creating this resource.
        name = '%s-%s' % (get_project(), get_stack())
        super(Stack, self).__init__('pulumi:pulumi:Stack', name, None, None)

        # Invoke the function while this stack is active and then register its outputs.
        self.outputs = dict()
        set_root_resource(self)
        try:
            func()
        finally:
            self.register_outputs(self.outputs)
            # Intentionally leave this resource installed in case subsequent async work uses it.

    def output(self, name: str, value: Any):
        """
        Export a stack output with a given name and value.
        """
        self.outputs[name] = value
