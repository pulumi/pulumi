import asyncio
import copy
from typing import Any, Awaitable, Callable, Mapping, Optional, TypeVar, Union


from .runtime.resource import register_resource_hook


class ResourceHookArgs:
    """
    ResourceHookArgs represents the arguments passed to a resource hook.

    Depending on the hook type, only some of the new/old inputs/outputs are set.

    | Hook Type     | old_inputs | new_inputs | old_outputs | new_outputs |
    | ------------- | ---------- | ---------- | ----------- | ----------- |
    | before_create |            | ✓          |             |             |
    | after_create  |            | ✓          |             | ✓           |
    | before_update | ✓          | ✓          | ✓           |             |
    | after_update  | ✓          | ✓          | ✓           | ✓           |
    | before_delete | ✓          |            | ✓           |             |
    | after_delete  | ✓          |            | ✓           |             |
    """

    urn: str
    """The URN of the resource that triggered the hook."""
    id: str
    """The ID of the resource that triggered the hook."""
    name: str
    """The name of the resource that triggered the hook."""
    type: str
    """The type of the resource that triggered the hook."""
    new_inputs: Optional[Mapping[str, Any]] = None
    """The new inputs of the resource that triggered the hook."""
    old_inputs: Optional[Mapping[str, Any]] = None
    """The old inputs of the resource that triggered the hook."""
    new_outputs: Optional[Mapping[str, Any]] = None
    """The new outputs of the resource that triggered the hook."""
    old_outputs: Optional[Mapping[str, Any]] = None
    """The old outputs of the resource that triggered the hook."""

    def __init__(
        self,
        urn: str,
        id: str,
        name: str,
        type: str,
        new_inputs: Optional[Mapping[str, Any]] = None,
        old_inputs: Optional[Mapping[str, Any]] = None,
        new_outputs: Optional[Mapping[str, Any]] = None,
        old_outputs: Optional[Mapping[str, Any]] = None,
    ):
        self.urn = urn
        self.id = id
        self.name = name
        self.type = type
        self.new_inputs = new_inputs
        self.old_inputs = old_inputs
        self.new_outputs = new_outputs
        self.old_outputs = old_outputs

    def __repr__(self):
        return (
            f"ResourceHookArgs(urn={self.urn}, "
            + f"id={self.id}, "
            + f"name={self.name}, "
            + f"type_={self.type}, "
            + f"new_inputs={self.new_inputs}, "
            + f"old_inputs={self.old_inputs}, "
            + f"new_outputs={self.new_outputs}, "
            + f"old_outputs={self.old_outputs})"
        )


ResourceHookFunction = Callable[
    [ResourceHookArgs],
    Union[None, Awaitable[None]],
]
"""ResourceHookFunction is a function that can be registered as a resource hook."""


class ResourceHookOptions:
    """Options for registering a resource hook."""

    on_dry_run: bool
    """Run the hook during dry run (preview) operations. Defaults to False."""

    def __init__(self, on_dry_run: bool = False):
        self.on_dry_run = on_dry_run

    def __repr__(self):
        return f"ResourceHookOptions(on_dry_run={self.on_dry_run})"


class ResourceHook:
    """ResourceHook is a named hook that can be registered as a resource hook."""

    name: str
    """The unqiue name of the resource hook."""
    callback: ResourceHookFunction
    """The function that will be called when the resource hook is triggered."""
    opts: Optional[ResourceHookOptions] = None
    _registered: asyncio.Future[None]
    """
    Tracks the registration of the resource hook. The future will resolve once
    the hook has been registered, or reject if any error occurs.
    """

    def __init__(
        self,
        name: str,
        func: ResourceHookFunction,
        opts: Optional[ResourceHookOptions] = None,
    ):
        self.__doc__ = func.__doc__
        self.__name__ = func.__name__
        self.name = name
        self.callback = func
        self.opts = opts
        self._registered = register_resource_hook(self)

    def __call__(self, args: ResourceHookArgs) -> Union[None, Awaitable[None]]:
        return self.callback(args)

    def __repr__(self) -> str:
        return f"ResourceHook(name={self.name}, callback={self.callback}, opts={self.opts})"


class ResourceHookBinding:
    """
    Binds :class:`ResourceHook` instances to a resource. The resource hooks will
    be invoked during certain step of the lifecycle of the resource.

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
    """
    Hooks to be invoked before the resource is deleted.

    Note that delete hooks require that destroy operations are run with `--run-program`. Unlike other hook types,
    this argument requires named :class:`ResourceHook` instances, and does not accept anonymous
    :class:`ResourceHookFunction`. This is because the engine needs to be able to identify a hook when a resource is
    deleted.
    """
    after_delete: Optional[list[ResourceHook]]
    """
    Hooks to be invoked after the resource is deleted.

    Note that delete hooks require that destroy operations are run with `--run-program`. Unlike other hook types,
    this argument requires named :class:`ResourceHook` instances, and does not accept anonymous
    :class:`ResourceHookFunction`. This is because the engine needs to be able to identify a hook when a resource is
    deleted.
    """

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
        return (
            f"ResourceHookBinding(before_create={self.before_create}, "
            + f"after_create={self.after_create}, "
            + f"before_update={self.before_update}, "
            + f"after_update={self.after_update}, "
            + f"before_delete={self.before_delete}, "
            + f"after_delete={self.after_delete})"
        )

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
