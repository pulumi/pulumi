import copy
from typing import Awaitable, Callable, Optional, TypeVar, Union


from .runtime.proto import callback_pb2
from .runtime.settings import _get_callbacks
from .runtime.sync_await import _sync_await

LifecycleHookFunction = Callable[
    # TODO all args, optionality?
    [str, str],  # urn, id
    None,
]


class LifecycleHook:
    name: str
    func: LifecycleHookFunction
    async_: Optional[bool] = False
    callback: Optional[callback_pb2.Callback] = None

    def __init__(
        self,
        name: str,
        func: LifecycleHookFunction,
        # TODO: opts: LifecycleHookOptions,
    ):
        self.__doc__ = func.__doc__
        self.__name__ = func.__name__
        self.name = name
        self.func = func

        callbacks = _sync_await(_get_callbacks())
        if callbacks is None:
            raise Exception("No callback server registered.")
        callbacks.register_lifecycle_hook(self)

    def __call__(self, urn: str, id: str) -> Union[None, Awaitable[None]]:
        self.func(urn, id)

    def __repr__(self):
        return f"<LifecycleHook {self.name} {self.func}>"


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
