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
from pulumi import ComponentResource, CustomResource, Output, InvokeOptions, ResourceOptions, log, Input
from typing import Callable, Any, Dict, List, Optional

from .remote import construct

class MyComponentArgs:
    input1: Input[int]
    def __init__(self,
                 input1: Input[int]) -> None:
        self.input1 = input1

class MyComponent(ComponentResource):
    myid: Output[str]
    output1: Output[int]
    # innerComponent: MyInnerComponent
    # nodeSecurityGroup: SecurityGroup
    def __init__(__self__, resource_name: str, opts: Optional[ResourceOptions]=None, input1=None) -> None:
        __props__ = dict()
        __props__["input1"] = input1
        if opts is None or opts.urn is None:
            async def do_construct():
                r = await construct("..", "MyComponent", resource_name, __props__, opts)
                return r["urn"]
            urn = asyncio.ensure_future(do_construct())
            opts = ResourceOptions.merge(opts, ResourceOptions(urn=urn))
        props = { 
            "input1": input1,
            "myid": None,
            "output1": None,
            "innerComponent": None,
            "nodeSecurityGroup": None,
        }
        super().__init__("my:mod:MyComponent", resource_name, props, opts)
            


            