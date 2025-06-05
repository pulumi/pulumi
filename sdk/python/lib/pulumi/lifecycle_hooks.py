import asyncio
import copy
from typing import Any, Awaitable, Callable, Optional, TypeVar, Union

import grpc

from . import log
from .runtime.proto import callback_pb2
from .runtime.settings import _get_callbacks, _get_rpc_manager, grpc_error_to_exception


class LifecycleHookArgs:
    urn: str
    id: str
    outputs: Optional[Any] = None

    def __init__(self, urn: str, id: str, outputs: Optional[Any] = None):
        self.urn = urn
        self.id = id
        self.outputs = outputs

    def __repr__(self):
        return f"LifecycleHookArgs(urn={self.urn}, id={self.id}, output={self.outputs})"


LifecycleHookFunction = Callable[
    [LifecycleHookArgs],
    Union[None, Awaitable[None]],
]


class LifecycleHookOptions:
    async_: bool

    def __init__(self, async_: bool = False):
        self.async_ = async_


class LifecycleHook:
    name: str
    func: LifecycleHookFunction
    callback: asyncio.Future[callback_pb2.Callback]
    opts: Optional[LifecycleHookOptions] = None

    def __init__(
        self,
        name: str,
        func: LifecycleHookFunction,
        opts: Optional[LifecycleHookOptions] = None,
    ):
        self.__doc__ = func.__doc__
        self.__name__ = func.__name__
        self.name = name
        self.func = func
        self.callback = asyncio.Future[callback_pb2.Callback]()

        async def do_register():
            try:
                callbacks = await _get_callbacks()
                log.debug(f"{callbacks=}")
                if callbacks is None:
                    raise Exception("No callback server registered.")
                self.callback.set_result(callbacks.register_lifecycle_hook(self))
            except Exception as e:  # noqa
                log.debug(f"Failed to register lifecycle hook {self.name}: {e}")
                if isinstance(e, grpc.RpcError):
                    self.callback.set_exception(grpc_error_to_exception(e))
                else:
                    self.callback.set_exception(e)

        asyncio.ensure_future(
            _get_rpc_manager().do_rpc("register lifecycle hook", do_register)()
        )

    def __call__(self, args: LifecycleHookArgs) -> Union[None, Awaitable[None]]:
        return self.func(args)

    def __repr__(self) -> str:
        return f"LifecycleHook(name={self.name}, func={self.func})"


class LifecycleHookBinding:
    before_create: Optional[list[Union[LifecycleHook, LifecycleHookFunction]]]
    after_create: Optional[list[Union[LifecycleHook, LifecycleHookFunction]]]
    before_update: Optional[list[Union[LifecycleHook, LifecycleHookFunction]]]
    after_update: Optional[list[Union[LifecycleHook, LifecycleHookFunction]]]
    before_delete: Optional[list[LifecycleHook]]
    after_delete: Optional[list[LifecycleHook]]

    def __init__(
        self,
        before_create: Optional[
            list[Union[LifecycleHook, LifecycleHookFunction]]
        ] = None,
        after_create: Optional[
            list[Union[LifecycleHook, LifecycleHookFunction]]
        ] = None,
        before_update: Optional[
            list[Union[LifecycleHook, LifecycleHookFunction]]
        ] = None,
        after_update: Optional[
            list[Union[LifecycleHook, LifecycleHookFunction]]
        ] = None,
        before_delete: Optional[list[LifecycleHook]] = None,
        after_delete: Optional[list[LifecycleHook]] = None,
    ):
        self.before_create = before_create
        self.after_create = after_create
        self.before_update = before_update
        self.after_update = after_update
        self.before_delete = before_delete
        self.after_delete = after_delete

    def __repr__(self):
        return f"<LifecycleHookBinding before_create={self.before_create}, after_create={self.after_create}, before_update={self.before_update}, after_update={self.after_update}, before_delete={self.before_delete}, after_delete={self.after_delete}>"

    def _copy(self):
        out = copy.copy(self)
        return out

    @classmethod
    def merge(
        cls,
        bindings1: Optional["LifecycleHookBinding"],
        bindings2: Optional["LifecycleHookBinding"],
    ) -> "LifecycleHookBinding":
        bindings1 = LifecycleHookBinding() if bindings1 is None else bindings1
        bindings2 = LifecycleHookBinding() if bindings2 is None else bindings2

        if not isinstance(bindings1, LifecycleHookBinding):
            raise TypeError("Expected bindings1 to be a LifecycleHookBinding instance")

        if not isinstance(bindings2, LifecycleHookBinding):
            raise TypeError("Expected bindings2 to be a LifecycleHookBinding instance")

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
