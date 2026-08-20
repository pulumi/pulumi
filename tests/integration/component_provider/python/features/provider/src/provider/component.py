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

from typing import TypedDict

import pulumi
from pulumi.runtime.proto import resource_pb2


class Args(TypedDict):
    pass


class MyComponent(pulumi.ComponentResource):
    parameterization: bool
    transforms: bool
    resourceHooks: bool

    def __init__(self, name: str, args: Args, opts: pulumi.ResourceOptions):
        super().__init__("provider:index:MyComponent", name, {}, opts)
        self.parameterization = (
            resource_pb2.RESOURCE_MONITOR_FEATURE_PARAMETERIZATION
            in pulumi.runtime.settings.SETTINGS.monitor_features
        )
        self.transforms = (
            resource_pb2.RESOURCE_MONITOR_FEATURE_TRANSFORMS
            in pulumi.runtime.settings.SETTINGS.monitor_features
        )
        self.resourceHooks = (
            resource_pb2.RESOURCE_MONITOR_FEATURE_RESOURCE_HOOKS
            in pulumi.runtime.settings.SETTINGS.monitor_features
        )
