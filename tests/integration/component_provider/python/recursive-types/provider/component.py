# Copyright 2025, Pulumi Corporation.
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


from typing import Optional, TypedDict

import pulumi


class RecursiveTypeA(TypedDict):
    b: Optional[pulumi.Input["RecursiveTypeB"]]


class RecursiveTypeB(TypedDict):
    a: Optional[pulumi.Input[RecursiveTypeA]]


class Args(TypedDict):
    rec: pulumi.Input[RecursiveTypeA]


class MyComponent(pulumi.ComponentResource):
    rec: pulumi.Output[RecursiveTypeB]

    def __init__(self, name: str, args: Args, opts: pulumi.ResourceOptions):
        super().__init__("component:index:MyComponent", name, {}, opts)
        self.rec = pulumi.Output.from_input({"a": args.get("rec")})
        self.register_outputs({})
