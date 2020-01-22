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
from typing import Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from .resource import Resource, ProviderResource

class InvokeOptions:
    """
    InvokeOptions is a bag of options that control the behavior of a call to runtime.invoke.
    """
    parent: Optional['Resource']
    """
    An optional parent to use for default options for this invoke (e.g. the default provider to use).
    """
    provider: Optional['ProviderResource']
    """
    An optional provider to use for this invocation. If no provider is supplied, the default provider for the
    invoked function's package will be used.
    """
    version: Optional[str]
    """
    An optional version. If provided, the provider plugin with exactly this version will be used to service
    the invocation.
    """

    def __init__(self,
                 parent: Optional['Resource'] = None,
                 provider: Optional['ProviderResource'] = None,
                 version: Optional[str] = "") -> None:
        """
        :param Optional[Resource] parent: An optional parent to use for default options for this invoke (e.g. the
               default provider to use).
        :param Optional[ProviderResource] provider: An optional provider to use for this invocation. If no provider is
               supplied, the default provider for the invoked function's package will be used.
        :param Optional[str] version: An optional version. If provided, the provider plugin with exactly this version
               will be used to service the invocation.
        """
        self.parent = parent
        self.provider = provider
        self.version = version
