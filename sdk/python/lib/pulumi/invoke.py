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
from typing import Optional, TYPE_CHECKING

if TYPE_CHECKING:
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
        version: Optional[str] = "",
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
        # Expose 'merge' again this this object, but this time as an instance method.
        # TODO[python/mypy#2427]: mypy disallows method assignment
        self.merge = self._merge_instance  # type: ignore
        self.merge.__func__.__doc__ = InvokeOptions.merge.__doc__  # type: ignore

        self.parent = parent
        self.provider = provider
        self.version = version
        self.plugin_download_url = plugin_download_url

    def _merge_instance(self, opts: "InvokeOptions") -> "InvokeOptions":
        return InvokeOptions.merge(self, opts)

    # pylint: disable=method-hidden
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
