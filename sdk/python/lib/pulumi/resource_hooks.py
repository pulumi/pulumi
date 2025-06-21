import asyncio
import copy
from typing import Any, Awaitable, Callable, Optional, TypeVar, Union

import grpc

from . import log
from .runtime.proto import callback_pb2
from .runtime.settings import _get_callbacks, _get_rpc_manager, grpc_error_to_exception


class ResourceHookArgs:
    urn: str
    id: str
    outputs: Optional[Any] = None

    def __init__(self, urn: str, id: str, outputs: Optional[Any] = None):
        self.urn = urn
        self.id = id
        self.outputs = outputs

    def __repr__(self):
        return f"ResourceHookArgs(urn={self.urn}, id={self.id}, output={self.outputs})"


ResourceHookFunction = Callable[
    [ResourceHookArgs],
    Union[None, Awaitable[None]],
]


class ResourceHookOptions:
    on_dry_run: bool
    """Run the hook during dry run operations. Defaults to False."""

    def __init__(self, on_dry_run: bool = False):
        self.on_dry_run = on_dry_run


class ResourceHook:
    name: str
    func: ResourceHookFunction
    callback: asyncio.Future[callback_pb2.Callback]
    opts: Optional[ResourceHookOptions] = None

    def __init__(
        self,
        name: str,
        func: ResourceHookFunction,
        opts: Optional[ResourceHookOptions] = None,
    ):
        self.__doc__ = func.__doc__
        self.__name__ = func.__name__
        self.name = name
        self.func = func
        self.opts = opts
        self.callback = asyncio.Future[callback_pb2.Callback]()

        async def do_register():
            try:
                callbacks = await _get_callbacks()
                if callbacks is None:
                    raise Exception("No callback server registered.")
                req = callbacks.register_resource_hook(self)
                self.callback.set_result(req.callback)
            except Exception as e:  # noqa
                if isinstance(e, grpc.RpcError):
                    e = grpc_error_to_exception(e)
                self.callback.set_exception(e)
                log.warn(f"Failed to register resource hook {self.name}: {e}")

        asyncio.ensure_future(
            _get_rpc_manager().do_rpc("register resource hook", do_register)()
        )

    def __call__(self, args: ResourceHookArgs) -> Union[None, Awaitable[None]]:
        return self.func(args)

    def __repr__(self) -> str:
        return f"ResourceHook(name={self.name}, func={self.func})"


class ResourceHookBinding:
    """
    Binds :class:`ResourceHook`s to a resource. The resource hooks will be
    invoked during certain step of the lifecycle of the resource.

    `before_${action}` hooks that raise an exception cause the action to fail.
    `after_${action}` hooks that raise an exception will log a warning, but do
    not cause the action or the deployment to fail.

    When running `pulumi destroy`, `before_delete` and `after_delete` resource
    hooks require the operation to run with `--run-program`, to ensure that the
    program which defines the hooks is available.
    """

    before_create: Optional[list[Union[ResourceHook, ResourceHookFunction]]]
    """Hooks to be invoked before the resource is created."""
    after_create: Optional[list[Union[ResourceHook, ResourceHookFunction]]]
    """Hooks to be invoked after the resource is created."""
    before_update: Optional[list[Union[ResourceHook, ResourceHookFunction]]]
    """Hooks to be invoked before the resource is updated."""
    after_update: Optional[list[Union[ResourceHook, ResourceHookFunction]]]
    """Hooks to be invoked after the resource is updated."""
    before_delete: Optional[list[ResourceHook]]
    """Hooks to be invoked before the resource is deleted."""
    after_delete: Optional[list[ResourceHook]]
    """Hooks to be invoked after the resource is deleted."""

    def __init__(
        self,
        before_create: Optional[list[Union[ResourceHook, ResourceHookFunction]]] = None,
        after_create: Optional[list[Union[ResourceHook, ResourceHookFunction]]] = None,
        before_update: Optional[list[Union[ResourceHook, ResourceHookFunction]]] = None,
        after_update: Optional[list[Union[ResourceHook, ResourceHookFunction]]] = None,
        before_delete: Optional[list[ResourceHook]] = None,
        after_delete: Optional[list[ResourceHook]] = None,
    ):
        self.before_create = before_create
        self.after_create = after_create
        self.before_update = before_update
        self.after_update = after_update
        self.before_delete = before_delete
        self.after_delete = after_delete

    def __repr__(self):
        return f"<ResourceHookBinding before_create={self.before_create}, after_create={self.after_create}, before_update={self.before_update}, after_update={self.after_update}, before_delete={self.before_delete}, after_delete={self.after_delete}>"

    def _copy(self):
        out = copy.copy(self)
        return out

    @classmethod
    def merge(
        cls,
        bindings1: Optional["ResourceHookBinding"],
        bindings2: Optional["ResourceHookBinding"],
    ) -> "ResourceHookBinding":
        bindings1 = ResourceHookBinding() if bindings1 is None else bindings1
        bindings2 = ResourceHookBinding() if bindings2 is None else bindings2

        if not isinstance(bindings1, ResourceHookBinding):
            raise TypeError("Expected bindings1 to be a ResourceHookBinding instance")

        if not isinstance(bindings2, ResourceHookBinding):
            raise TypeError("Expected bindings2 to be a ResourceHookBinding instance")

        dest = bindings1._copy()
        source = bindings2._copy()

        dest.before_create = _merge_lists(dest.before_create, source.before_create)
        dest.after_create = _merge_lists(dest.after_create, source.after_create)
        dest.before_update = _merge_lists(dest.before_update, source.before_update)
        dest.after_update = _merge_lists(dest.after_update, source.after_update)
        dest.before_delete = _merge_lists(dest.before_delete, source.before_delete)
        dest.after_delete = _merge_lists(dest.after_delete, source.after_delete)

        return dest


T = TypeVar("T")


def _merge_lists(dest: Optional[list[T]], source: Optional[list[T]]):
    if dest is None:
        return source

    if source is None:
        return dest

    return dest + source
