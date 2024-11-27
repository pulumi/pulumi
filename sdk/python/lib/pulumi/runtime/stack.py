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
from typing import Any, Callable, Dict, List, Awaitable, Optional

from . import settings
from .. import log
from ..resource import (
    ComponentResource,
    Resource,
    ResourceTransformation,
    ResourceTransform,
)
from ..invoke import (
    InvokeTransform,
)
from .settings import (
    SETTINGS,
    _get_callbacks,
    _get_rpc_manager,
    _load_monitor_feature_support,
    _shutdown_callbacks,
    _sync_monitor_supports_transforms,
    _sync_monitor_supports_invoke_transforms,
    get_project,
    get_root_resource,
    get_stack,
    is_dry_run,
    set_root_resource,
)
from .sync_await import _sync_await


async def run_pulumi_func(func: Callable[[], None]):
    try:
        func()
    finally:
        await wait_for_rpcs()
        await _shutdown_callbacks()

        # By now, all tasks have exited and we're good to go.
        log.debug("run_pulumi_func completed")


async def wait_for_rpcs(await_all_outstanding_tasks=True) -> None:
    log.debug("Waiting for outstanding RPCs to complete")

    rpc_manager = _get_rpc_manager()
    while True:
        # Pump the event loop, giving all of the RPCs that we just queued up time to fully execute.
        # The asyncio scheduler does not expose a "yield" primitive, so this will have to do.
        #
        # Note that "asyncio.sleep(0)" is the blessed way to do this:
        # https://github.com/python/asyncio/issues/284#issuecomment-154180935
        #
        # We await each RPC in turn so that this loop will actually block rather than busy-wait.
        while len(rpc_manager.rpcs) > 0:
            await asyncio.sleep(0)
            if settings.excessive_debug_output:
                log.debug(
                    f"waiting for quiescence; {len(rpc_manager.rpcs)} RPCs outstanding"
                )
            try:
                await rpc_manager.rpcs.pop()
            except Exception as exn:
                # If the RPC failed, re-raise the original traceback
                # instead of the await above.
                if rpc_manager.unhandled_exception is not None:
                    cause = rpc_manager.unhandled_exception.with_traceback(
                        rpc_manager.exception_traceback,
                    )
                    raise exn from cause

                raise

        if rpc_manager.unhandled_exception is not None:
            raise rpc_manager.unhandled_exception.with_traceback(
                rpc_manager.exception_traceback
            )

        log.debug("RPCs successfully completed")

        # If the RPCs have successfully completed, now await all remaining outstanding tasks.
        if await_all_outstanding_tasks:
            while len(SETTINGS.outputs) != 0:
                await asyncio.sleep(0)
                if settings.excessive_debug_output:
                    log.debug(
                        f"waiting for quiescence; {len(SETTINGS.outputs)} outputs outstanding"
                    )
                with SETTINGS.lock:
                    # the task may have been removed from the queue by the time we get to it, so we need to re-check if
                    # its empty.
                    if len(SETTINGS.outputs) == 0:
                        break
                    task: asyncio.Task = SETTINGS.outputs.popleft()

                # check if the task is ready yet, else just add it back to the queue. This is so if a long running task
                # is added to the queue first, then a short running task that fails is added to the queue we quickly see
                # that short running failure and exit, not waiting for the long running task to complete.
                if task.done():
                    await task
                else:
                    with SETTINGS.lock:
                        SETTINGS.outputs.append(task)

            log.debug("All outstanding outputs completed.")

        # Check to see if any more RPCs or outputs have been scheduled, and repeat the cycle if so.
        # Break if no RPCs remain.
        if len(rpc_manager.rpcs) == 0:
            break


async def run_in_stack(func: Callable[[], Optional[Awaitable[None]]]):
    """
    Run the given function inside of a new stack resource.  This ensures that any stack export calls
    will end up as output properties on the resulting stack component in the checkpoint file.  This
    is meant for internal runtime use only and is used by the Python SDK entrypoint program.
    """

    def run() -> None:
        Stack(func)

    await _load_monitor_feature_support()
    await run_pulumi_func(run)


class Stack(ComponentResource):
    """
    A synthetic stack component that automatically parents resources as the program runs.
    """

    outputs: Dict[str, Any]

    def __init__(self, func: Callable[[], Optional[Awaitable[None]]]) -> None:
        # Ensure we don't already have a stack registered.
        if get_root_resource() is not None:
            raise Exception("Only one root Pulumi Stack may be active at once")

        # Now invoke the registration to begin creating this resource.
        name = f"{get_project()}-{get_stack()}"
        super().__init__("pulumi:pulumi:Stack", name, None, None)

        # Invoke the function while this stack is active and then register its outputs. func might return an awaitable
        # so we need to await it, ideally we'd do this in a standard way but alas back compatibility means we do
        # everything in stack constructors, so we have to use sync_await here.

        self.outputs = {}
        set_root_resource(self)
        try:
            awaitable = func()
            # This _should_ be an awaitable but old pulumi executors returned modules here, so we need to handle that
            # with a type check rather than just `is not None`.
            if isawaitable(awaitable):
                _sync_await(awaitable)
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
    from .. import Output

    # Basic primitive types (numbers, booleans, strings, etc.) don't need any special handling.
    if is_primitive(attr):
        return attr

    if isinstance(attr, Output):
        return attr.apply(lambda v: massage(v, seen))

    if isawaitable(attr):
        return Output.from_input(attr).apply(lambda v: massage(v, seen))

    # from this point on, we have complex objects.  If we see them again, we don't want to emit them
    # again fully or else we'd loop infinitely.
    if reference_contains(attr, seen):
        # Note: for Resources we hit again, emit their urn so cycles can be easily understood in
        # the popo objects.
        if isinstance(attr, Resource):
            return massage(attr.urn, seen)
        # otherwise just emit as nothing to stop the looping.
        return None

    try:
        seen.append(attr)
        return massage_complex(attr, seen)
    finally:
        popped = seen.pop()
        if popped is not attr:
            raise Exception("Invariant broken when processing stack outputs")


def massage_complex(attr: Any, seen: List[Any]) -> Any:
    def is_public_key(key: str) -> bool:
        return not key.startswith("_")

    def serialize_all_keys(include: Callable[[str], bool]):
        plain_object: Dict[str, Any] = {}
        for key in attr.__dict__.keys():
            if include(key):
                plain_object[key] = massage(attr.__dict__[key], seen)
        return plain_object

    if isinstance(attr, Resource):
        serialized_attr = serialize_all_keys(is_public_key)

        # In preview only, we mark the result with "@isPulumiResource" to indicate that it is derived
        # from a resource. This allows the engine to perform resource-specific filtering of unknowns
        # from output diffs during a preview. This filtering is not necessary during an update because
        # all property values are known.
        return (
            serialized_attr
            if not is_dry_run()
            else {**serialized_attr, "@isPulumiResource": True}
        )

    # first check if the value is an actual dictionary.  If so, massage the values of it to deeply
    # make sure this is a popo.
    if isinstance(attr, dict):
        # Don't use attr.items() here, as it will error in the case of outputs with an `items` property.
        return {
            key: massage(attr[key], seen) for key in attr if not key.startswith("_")
        }

    if hasattr(attr, "__iter__"):
        return [massage(item, seen) for item in attr]

    return serialize_all_keys(is_public_key)


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
        raise Exception(
            "The root stack resource was referenced before it was initialized."
        )
    if root_resource._transformations is None:
        root_resource._transformations = [t]
    else:
        root_resource._transformations = root_resource._transformations + [t]


def register_resource_transform(t: ResourceTransform) -> None:
    """
    Add a transform to all future resources constructed in this Pulumi stack.
    """
    if not _sync_monitor_supports_transforms():
        raise Exception(
            "The Pulumi CLI does not support transforms. Please update the Pulumi CLI."
        )

    # We need to make sure all the current resource registrations are finished before
    # registering the transforms.  Do so by waiting for all RPCs to complete, before
    # we go ahead and register the transform.
    pending = asyncio.all_tasks()
    rpcs = {task for task in pending if task.get_coro().__name__ == "rpc_wrapper"}  # type: ignore
    _sync_await(asyncio.gather(*rpcs))

    callbacks = _sync_await(_get_callbacks())
    if callbacks is None:
        raise Exception("No callback server registered.")
    callbacks.register_stack_transform(t)


def register_stack_transform(t: ResourceTransform):
    """
    Add a transform to all future resources constructed in this Pulumi stack.

    Deprecated: use `register_resource_transform` instead.
    """
    register_resource_transform(t)


def register_invoke_transform(t: InvokeTransform) -> None:
    """
    Add a transforms to all future invokes called in this Pulumi stack.
    """

    if not _sync_monitor_supports_invoke_transforms():
        raise Exception(
            "The Pulumi CLI does not support invoke transforms. Please update the Pulumi CLI."
        )

    # We need to make sure all the current invokes are finished before
    # registering the transforms.  Do so by waiting for all RPCs to
    # complete, before we go ahead and register the transform.
    pending = asyncio.all_tasks()
    rpcs = {task for task in pending if task.get_coro().__name__ == "rpc_wrapper"}  # type: ignore
    _sync_await(asyncio.gather(*rpcs))

    callbacks = _sync_await(_get_callbacks())
    if callbacks is None:
        raise Exception("No callback server registered.")
    callbacks.register_invoke_transform(t)
