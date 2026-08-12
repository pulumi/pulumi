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

import sys
import os
from typing import TypedDict

import pulumi


# Make the mock packages available
sys.path.insert(0, os.path.join(os.path.dirname(__file__)))
import mock_package
import mock_package_para


class Args(TypedDict):
    res: pulumi.Input[mock_package.MyResource]
    res_para: pulumi.Input[mock_package_para.MyResource]
    enum: pulumi.Input[mock_package.MyEnum]
    enum_para: pulumi.Input[mock_package_para.MyEnum]


class Component(pulumi.ComponentResource):
    def __init__(self, args: Args): ...
