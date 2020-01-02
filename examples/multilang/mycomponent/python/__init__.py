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
from pulumi import ComponentResource, Output,  ResourceOptions, Input, Inputs
from pulumi.runtime import register_proxy_constructor
from typing import Callable, Any, Dict, List, Optional
from pulumi_aws import ec2

from .remote import construct

class ProxyComponentResource(ComponentResource):
    """
    Abstract base class for proxies around component resources.

    TODO: This should move into the core Python SDK.
    """
    def __init__(__self__,
                 t: str,
                 name: str,
                 library_path: str,
                 library_name: str,
                 inputs: Inputs,
                 outputs: Dict[str, None],
                 opts: Optional[ResourceOptions]=None) -> None:
        if opts is None or opts.urn is None:
            async def do_construct():
                r = await construct(library_path, library_name, name, inputs, opts)
                return r["urn"]
            urn = asyncio.ensure_future(do_construct())
            opts = ResourceOptions.merge(opts, ResourceOptions(urn=urn))
        props = {
            **inputs,
            **outputs,
        }
        super().__init__(t, name, props, opts)

class MyInnerComponent(ProxyComponentResource):
    data: Output[str]
    def __init__(__self__, resource_name: str, opts: Optional[ResourceOptions]=None) -> None:
        super().__init__(
            "my:mod:MyInnerComponent",
            resource_name,
            "..",
            "MyInnerComponent",
            {},
            {
                "data": None
            },
            opts,
        )
register_proxy_constructor("my:mod:MyInnerComponent", lambda name, opts: MyInnerComponent(name, ResourceOptions(**opts)))

class MyComponent(ProxyComponentResource):
    myid: Output[str]
    output1: Output[int]
    innerComponent: MyInnerComponent
    nodeSecurityGroup: ec2.SecurityGroup
    def __init__(__self__, resource_name: str, opts: Optional[ResourceOptions]=None, input1:Optional[Input]=None) -> None:
        super().__init__(
            "my:mod:MyComponent",
            resource_name,
            "..",
            "MyComponent",
            {
                "input1": input1
            },
            {
                "myid": None,
                "output1": None,
                "innerComponent": None,
                "nodeSecurityGroup": None
            },
            opts,
        )
register_proxy_constructor("my:mod:MyComponent", MyComponent)
