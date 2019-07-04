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

import asyncio
import base64
import pickle
from typing import Optional, TYPE_CHECKING

import dill
from .. import CustomResource

if TYPE_CHECKING:
    from .. import ResourceOptions
    from ..output import Output, Inputs

PROVIDER_KEY = "__provider"

class ResourceProvider:
    """
    ResourceProvider is a Dynamic Resource Provider which allows defining new kinds of resources
    whose CRUD operations are implemented inside your Python program.
    """

    def check(self, _olds, news):
        return {'inputs': news, 'failures': []}
    def diff(self, _id, _olds, _news):
        return {}
    def create(self, props):
        raise Exception("Subclass of ResourceProvider must implement 'create'")
    def read(self, id_, props):
        return {'id': id_, 'props': props}
    def update(self, _id, _olds, _news):
        return {}
    def delete(self, _id, _props):
        pass
    def __init__(self):
        pass

def serialize_provider(provider: ResourceProvider) -> str:
    byts = dill.dumps(lambda: provider, pickle.DEFAULT_PROTOCOL)
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
