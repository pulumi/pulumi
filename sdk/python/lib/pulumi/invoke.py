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
import copy
from typing import (
    Any,
    Awaitable,
    Callable,
    List,
    Optional,
    Sequence,
    TYPE_CHECKING,
    Union,
    cast,
)

if TYPE_CHECKING:
    from .output import Inputs, Input
    from .resource import Resource, ProviderResource


class InvokeOptions:
    """
    InvokeOptions is a bag of options that control the behavior of a call to runtime.invoke.
    """

    parent: Optional["Resource"]
    """
    An optional parent to use for default options for this invoke (e.g. the default provider to use).
    """
    provider: Optional["ProviderResource"]
    """
    An optional provider to use for this invocation. If no provider is supplied, the default provider for the
    invoked function's package will be used.
    """
    version: Optional[str]
    """
    An optional version. If provided, the provider plugin with exactly this version will be used to service
    the invocation.
    """
    plugin_download_url: Optional[str]
    """
    An optional URL. If provided, the provider plugin with exactly this download URL will be used to service
    the invocation. This will override the URL sourced from the host package, and should be rarely used.
    """

    def __init__(
        self,
        parent: Optional["Resource"] = None,
        provider: Optional["ProviderResource"] = None,
        version: Optional[str] = None,
        plugin_download_url: Optional[str] = None,
    ) -> None:
        """
        :param Optional[Resource] parent: An optional parent to use for default options for this invoke (e.g. the
               default provider to use).
        :param Optional[ProviderResource] provider: An optional provider to use for this invocation. If no provider is
               supplied, the default provider for the invoked function's package will be used.
        :param Optional[str] version: An optional version. If provided, the provider plugin with exactly this version
               will be used to service the invocation.
        :param Optional[str] plugin_download_url: An optional URL. If provided, the provider plugin with this download
               URL will be used to service the invocation. This will override the URL sourced from the host package, and
               should be rarely used.
        """
        # Expose 'merge' on this object as an instance method.
        # TODO[python/mypy#2427]: mypy disallows method assignment
        self.merge = self._merge_instance  # type: ignore
        self.merge.__func__.__doc__ = InvokeOptions.merge.__doc__  # type: ignore

        self.parent = parent
        self.provider = provider
        self.version = version
        self.plugin_download_url = plugin_download_url

    def _merge_instance(self, opts: "InvokeOptions") -> "InvokeOptions":
        return InvokeOptions.merge(self, opts)

    @staticmethod
    def merge(
        opts1: Optional["InvokeOptions"],
        opts2: Optional["InvokeOptions"],
    ) -> "InvokeOptions":
        """
        merge produces a new InvokeOptions object with the respective attributes of the `opts1`
        instance in it with the attributes of `opts2` merged over them.

        Both the `opts1` instance and the `opts2` instance will be unchanged.  Both of `opts1` and
        `opts2` can be `None`, in which case its attributes are ignored.

        Conceptually attributes merging follows these basic rules:

        1. If the attributes is a collection, the final value will be a collection containing the
           values from each options object. Both original collections in each options object will
           be unchanged.

        2. Simple scalar values from `opts2` (i.e. strings, numbers, bools) will replace the values
           from `opts1`.

        3. For the purposes of merging `depends_on` is always treated
           as collections, even if only a single value was provided.

        4. Attributes with value 'None' will not be copied over.

        This method can be called either as static-method like `InvokeOptions.merge(opts1, opts2)`
        or as an instance-method like `opts1.merge(opts2)`.  The former is useful for cases where
        `opts1` may be `None` so the caller does not need to check for this case.
        """
        opts1 = InvokeOptions() if opts1 is None else opts1
        opts2 = InvokeOptions() if opts2 is None else opts2

        if not isinstance(opts1, InvokeOptions):
            raise TypeError("Expected opts1 to be a InvokeOptions instance")

        if not isinstance(opts2, InvokeOptions):
            raise TypeError("Expected opts2 to be a InvokeOptions instance")

        dest = copy.copy(opts1)
        source = opts2

        dest.parent = dest.parent if source.parent is None else source.parent
        dest.provider = dest.provider if source.provider is None else source.provider
        dest.plugin_download_url = (
            dest.plugin_download_url
            if source.plugin_download_url is None
            else source.plugin_download_url
        )
        dest.version = dest.version if source.version is None else source.version

        return dest


class InvokeOutputOptions(InvokeOptions):
    """
    InvokeOutputOptions is a bag of options that control the behavior of a call to runtime.invoke_output.
    """

    depends_on: Optional["Input[Union[Sequence[Input[Resource]], Resource]]"]
    """
    If provided, declares that the currently-executing invoke depends on the given resources.
    """

    def __init__(
        self,
        parent: Optional["Resource"] = None,
        provider: Optional["ProviderResource"] = None,
        version: Optional[str] = "",
        plugin_download_url: Optional[str] = None,
        depends_on: Optional[
            "Input[Union[Sequence[Input[Resource]], Resource]]"
        ] = None,
    ) -> None:
        super().__init__(
            parent=parent,
            provider=provider,
            version=version,
            plugin_download_url=plugin_download_url,
        )
        self.depends_on = depends_on

        # Expose 'merge' on this object as an instance method.
        # TODO[python/mypy#2427]: mypy disallows method assignment
        self.merge = self._merge_instance  # type: ignore
        self.merge.__func__.__doc__ = InvokeOptions.merge.__doc__  # type: ignore

    def _merge_instance(
        self, opts: "Union[InvokeOptions, InvokeOutputOptions]"
    ) -> "InvokeOutputOptions":
        return InvokeOutputOptions.merge(self, opts)

    @staticmethod
    def merge(
        opts1: Optional[Union["InvokeOptions", "InvokeOutputOptions"]],
        opts2: Optional[Union["InvokeOptions", "InvokeOutputOptions"]],
    ) -> "InvokeOutputOptions":
        """
        merge produces a new InvokeOutputOptions object with the respective attributes of the `opts1`
        instance in it with the attributes of `opts2` merged over them.

        Both the `opts1` instance and the `opts2` instance will be unchanged.  Both of `opts1` and
        `opts2` can be `None`, in which case its attributes are ignored.

        Conceptually attributes merging follows these basic rules:

        1. If the attributes is a collection, the final value will be a collection containing the
            values from each options object. Both original collections in each options object will
            be unchanged.

        2. Simple scalar values from `opts2` (i.e. strings, numbers, bools) will replace the values
            from `opts1`.

        3. For the purposes of merging `depends_on` is always treated
            as collections, even if only a single value was provided.

        4. Attributes with value 'None' will not be copied over.

        This method can be called either as static-method like `InvokeOutputOptions.merge(opts1, opts2)`
        or as an instance-method like `opts1.merge(opts2)`.  The former is useful for cases where
        `opts1` may be `None` so the caller does not need to check for this case.
        """
        opts1 = InvokeOutputOptions() if opts1 is None else opts1
        opts2 = InvokeOutputOptions() if opts2 is None else opts2

        if not isinstance(opts1, InvokeOptions) and not isinstance(
            opts1, InvokeOutputOptions
        ):
            raise TypeError(
                "Expected opts1 to be an InvokeOptions or InvokeOutputOptions instance"
            )

        if not isinstance(opts2, InvokeOptions) and not isinstance(
            opts2, InvokeOutputOptions
        ):
            raise TypeError(
                "Expected opts2 to be an InvokeOptions or InvokeOutputOptions instance"
            )

        dest = InvokeOutputOptions(
            parent=opts1.parent,
            provider=opts1.provider,
            version=opts1.version,
            plugin_download_url=opts1.plugin_download_url,
            depends_on=(
                opts1.depends_on if isinstance(opts1, InvokeOutputOptions) else None
            ),
        )

        source = opts2

        dest.parent = dest.parent if source.parent is None else source.parent
        dest.provider = dest.provider if source.provider is None else source.provider
        dest.plugin_download_url = (
            dest.plugin_download_url
            if source.plugin_download_url is None
            else source.plugin_download_url
        )
        dest.version = (
            dest.version if (source.version in [None, ""]) else source.version
        )
        if isinstance(source, InvokeOutputOptions):
            # Avoid circular import
            from .output import _map2_input

            dest.depends_on = _map2_input(
                dest._depends_on_list(),
                source._depends_on_list(),
                lambda xs, ys: xs + ys,
            )

        return dest

    def _depends_on_list(self) -> "Input[List[Input[Resource]]]":
        if self.depends_on is None:
            return []

        # Avoid circular import
        from .output import _map_input

        return _map_input(
            self.depends_on,
            lambda x: list(x) if isinstance(x, Sequence) else [cast(Any, x)],
        )


class InvokeTransformArgs:
    """
    InvokeTransformArgs is the argument bag passed to an invoke transform.
    """

    token: str
    """
    The token of the invoke.
    """

    args: "Inputs"
    """
    The original arguments passed to the invocation.
    """

    opts: "InvokeOptions"
    """
    The original invoke options passed to the invocation.
    """

    def __init__(
        self,
        token: str,
        args: "Inputs",
        opts: "InvokeOptions",
    ) -> None:
        self.token = token
        self.args = args
        self.opts = opts


class InvokeTransformResult:
    """
    InvokeTransformResult is the result that must be returned by an invoke transform callback.
    It includes new values to use for the `args` and `opts` of the `Invoke` in place of the
    originally provided values.
    """

    args: "Inputs"
    """
    The new arguments to use in place of the original `args`.
    """

    opts: "InvokeOptions"
    """
    The new invoke options to use in place of the original `opts`.
    """

    def __init__(self, args: "Inputs", opts: "InvokeOptions") -> None:
        self.args = args
        self.opts = opts


InvokeTransform = Callable[
    [InvokeTransformArgs],
    Optional[Union[Awaitable[Optional[InvokeTransformResult]], InvokeTransformResult]],
]
"""
InvokeTransform is the callback signature for the `transforms` invoke option.  A
transform is passed the same set of inputs provided to the `Invoke` constructor, and can
optionally return back alternate values for the `args` and/or `opts` prior to the invoke
actually being called.  The effect will be as though those args and opts were passed in place
of the original call to the `Invoke` call.  If the transform returns None,
this indicates that the invoke will not be transformed.
"""
