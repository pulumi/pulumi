# Copyright 2026, Pulumi Corporation.
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

# A canned "generated" component SDK used as an external base in analyzer tests. Its Service carries a pulumi_type
# marker (via the type_token decorator) exactly as codegen would stamp it.

from typing import TypedDict

import pulumi


class ServiceArgs(TypedDict):
    image: pulumi.Input[str]


@pulumi.type_token("basecomp:index:Service")
class Service(pulumi.ComponentResource):
    address: pulumi.Output[str]

    def __init__(self, args: ServiceArgs): ...
