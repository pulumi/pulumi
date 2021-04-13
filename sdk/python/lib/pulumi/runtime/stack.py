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
from inspect import isawaitable
from typing import Callable, Any, Dict, List, TYPE_CHECKING

from ..resource import ComponentResource, Resource, ResourceTransformation
from .settings import get_project, get_stack, get_root_resource, is_dry_run, set_root_resource
from .rpc_manager import RPC_MANAGER
from .sync_await import _all_tasks, _get_current_task
from .. import log

if TYPE_CHECKING:
    from .. import Output


def _get_running_tasks() -> List[asyncio.Task]:
    pending = []
    for task in _all_tasks():
        # Don't kill ourselves, that would be silly.
        if not task == _get_current_task():
            pending.append(task)
    return pending


async def run_pulumi_func(func: Callable):
    try:
        func()
    finally:
        await wait_for_rpcs()

        # By now, all tasks have exited and we're good to go.
        log.debug("run_pulumi_func completed")


async def wait_for_rpcs(await_all_outstanding_tasks=True) -> None:
    log.debug("Waiting for outstanding RPCs to complete")

    while True:
        # Pump the event loop, giving all of the RPCs that we just queued up time to fully execute.
        # The asyncio scheduler does not expose a "yield" primitive, so this will have to do.
        #
        # Note that "asyncio.sleep(0)" is the blessed way to do this:
        # https://github.com/python/asyncio/issues/284#issuecomment-154180935
        #
        # We await each RPC in turn so that this loop will actually block rather than busy-wait.
        while len(RPC_MANAGER.rpcs) > 0:
            await asyncio.sleep(0)
            log.debug(f"waiting for quiescence; {len(RPC_MANAGER.rpcs)} RPCs outstanding")
            await RPC_MANAGER.rpcs.pop()

        if RPC_MANAGER.unhandled_exception is not None:
            raise RPC_MANAGER.unhandled_exception.with_traceback(RPC_MANAGER.exception_traceback)

        log.debug("RPCs successfully completed")

        # If the RPCs have successfully completed, now await all remaining outstanding tasks.
        if await_all_outstanding_tasks:

            outstanding_tasks = _get_running_tasks()
            if len(outstanding_tasks) == 0:
                log.debug("No outstanding tasks to complete")
            else:
                log.debug(f"Waiting for {len(outstanding_tasks)} outstanding tasks to complete")

                done, pending = await asyncio.wait(outstanding_tasks, return_when="FIRST_EXCEPTION")

                if len(pending) > 0:
                    # If there are any pending tasks, it's because an exception was thrown.
                    # Cancel any pending tasks.
                    log.debug(f"Cancelling {len(pending)} remaining tasks.")
                    for task in pending:
                        task.cancel()

                for task in done:
                    exception = task.exception()
                    if exception is not None:
                        log.debug("A future resolved in an exception, raising exception.")
                        raise exception

                log.debug("All outstanding tasks completed.")

        # Check to see if any more RPCs have been scheduled, and repeat the cycle if so.
        # Break if no RPCs remain.
        if len(RPC_MANAGER.rpcs) == 0:
            break


async def run_in_stack(func: Callable):
    """
    Run the given function inside of a new stack resource.  This ensures that any stack export calls
    will end up as output properties on the resulting stack component in the checkpoint file.  This
    is meant for internal runtime use only and is used by the Python SDK entrypoint program.
    """
    await run_pulumi_func(lambda: Stack(func))


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
        super().__init__('pulumi:pulumi:Stack', name, None, None)

        # Invoke the function while this stack is active and then register its outputs.
        self.outputs = dict()
        set_root_resource(self)
        try:
            func()
        finally:
            self.register_outputs(massage(self.outputs, []))
            # Intentionally leave this resource installed in case subsequent async work uses it.

    def output(self, name: str, value: Any):
        """
        Export a stack output with a given name and value.
        """
        self.outputs[name] = value


# Note: we use a List here instead of a set as many objects are unhashable.  This is inefficient,
# but python seems to offer no alternative.
def massage(attr: Any, seen: List[Any]):
    """
    massage takes an arbitrary python value and attempts to *deeply* convert it into
    plain-old-python-value that can registered as an output.  In general, this means leaving alone
    things like strings, ints, bools. However, it does mean trying to make other values into either
    lists or dictionaries as appropriate.  In general, iterable things are turned into lists, and
    dictionary-like things are turned into dictionaries.
    """
    from .. import Output  # pylint: disable=import-outside-toplevel

    # Basic primitive types (numbers, booleans, strings, etc.) don't need any special handling.
    if is_primitive(attr):
        return attr

    # from this point on, we have complex objects.  If we see them again, we don't want to emit them
    # again fully or else we'd loop infinitely.
    if reference_contains(attr, seen):
        # Note: for Resources we hit again, emit their urn so cycles can be easily understood in
        # the popo objects.
        if isinstance(attr, Resource):
            return attr.urn

        # otherwise just emit as nothing to stop the looping.
        return None

    seen.append(attr)

    # first check if the value is an actual dictionary.  If so, massage the values of it to deeply
    # make sure this is a popo.
    if isinstance(attr, dict):
        result = {}
        # Don't use attr.items() here, as it will error in the case of outputs with an `items` property.
        for key in attr:
            # ignore private keys
            if not key.startswith("_"):
                result[key] = massage(attr[key], seen)

        return result

    if isinstance(attr, Output):
        return attr.apply(lambda v: massage(v, seen))

    if isawaitable(attr):
        return Output.from_input(attr).apply(lambda v: massage(v, seen))

    if isinstance(attr, Resource):
        result = massage(attr.__dict__, seen)

        # In preview only, we mark the result with "@isPulumiResource" to indicate that it is derived
        # from a resource. This allows the engine to perform resource-specific filtering of unknowns
        # from output diffs during a preview. This filtering is not necessary during an update because
        # all property values are known.
        if is_dry_run():
            result["@isPulumiResource"] = True
        return result

    if hasattr(attr, "__dict__"):
        # recurse on the dictionary itself.  It will be handled above.
        return massage(attr.__dict__, seen)

    # finally, recurse through iterables, converting into a list of massaged values.
    return [massage(a, seen) for a in attr]


def reference_contains(val1: Any, seen: List[Any]) -> bool:
    for val2 in seen:
        if val1 is val2:
            return True

    return False


def is_primitive(attr: Any) -> bool:
    if attr is None:
        return True

    if isinstance(attr, str):
        return True

    # dictionaries, lists and dictionary-like things are not primitive.
    if isinstance(attr, dict):
        return False

    if hasattr(attr, "__dict__"):
        return False

    try:
        iter(attr)
        return False
    except TypeError:
        pass

    return True


def register_stack_transformation(t: ResourceTransformation):
    """
    Add a transformation to all future resources constructed in this Pulumi stack.
    """
    root_resource = get_root_resource()
    if root_resource is None:
        raise Exception("The root stack resource was referenced before it was initialized.")
    if root_resource._transformations is None:
        root_resource._transformations = [t]
    else:
        root_resource._transformations = root_resource._transformations + [t]
