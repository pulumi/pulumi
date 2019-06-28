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

import base64
import asyncio
from typing import Optional, List, Any, Mapping, Union, TYPE_CHECKING, Awaitable

from .. import CustomResource, ResourceOptions
from ..runtime import closure

if TYPE_CHECKING:
    from ..output import Output, Inputs

PROVIDER_KEY = "__provider"

class ResourceProvider:
    def check(self, _olds, news):
        print("Calling Check!!!!")
        return {'inputs': news, 'failures': []}
    def __init__(self):
        pass

def serialize_provider(provider: ResourceProvider) -> str:
    byts = closure.serialize_function(lambda: provider)
    return base64.b64encode(byts).decode('utf-8')

class Resource(CustomResource):
    """
    Resource represents a Pulumi Resource that incorporates an inline implementation of the Resource's CRUD operations.
    """

    def __init__(self,
                 provider: ResourceProvider,
                 name: str,
                 props: 'Inputs',
                 opts: Optional[ResourceOptions] = None) -> None:
        """
        :param str provider: The implementation of the resource's CRUD operations.
        :param str name: The name of this resource.
        :param Optional[dict] props: The arguments to use to populate the new resource. Must not define the reserved
                property "__provider".
        :param Optional[ResourceOptions] opts: A bag of options that control this resource's behavior.
        """

        if PROVIDER_KEY in props:
            raise  Exception("A dynamic resource must not define the __provider key")
        
        props[PROVIDER_KEY] = serialize_provider(provider)

        super(Resource, self).__init__("pulumi-python:dynamic:Resource", name, props, opts)

