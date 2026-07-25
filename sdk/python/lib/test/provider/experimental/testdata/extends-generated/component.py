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

import os
import sys

import pulumi

# Make the canned base package available.
sys.path.insert(0, os.path.join(os.path.dirname(__file__)))
import base_component


class MyServiceArgs(base_component.ServiceArgs):
    replicas: pulumi.Input[int]


class MyService(base_component.Service):
    endpoint: pulumi.Output[str]

    def __init__(self, args: MyServiceArgs): ...
