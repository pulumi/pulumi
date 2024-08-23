# Copyright 2016-2024, Pulumi Corporation.
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

from typing import Any, Mapping, Optional, Sequence

import pulumi


@pulumi.input_type
class IdentityPropertiesArgs:
    def __init__(__self__, *,
                 user_assigned_identities: Optional[pulumi.Input[Sequence[pulumi.Input[str]]]] = None):
        if user_assigned_identities is not None:
            pulumi.set(__self__, "user_assigned_identities", user_assigned_identities)

    @property
    @pulumi.getter(name="userAssignedIdentities")
    def user_assigned_identities(self) -> Optional[pulumi.Input[Sequence[pulumi.Input[str]]]]:
        return pulumi.get(self, "user_assigned_identities")

    @user_assigned_identities.setter
    def user_assigned_identities(self, value: Optional[pulumi.Input[Sequence[pulumi.Input[str]]]]):
        pulumi.set(self, "user_assigned_identities", value)


@pulumi.output_type
class UserAssignedIdentityResponse(dict):
    def __getitem__(self, key: str) -> Any:
        return super().__getitem__(key)

    def get(self, key: str, default = None) -> Any:
        return super().get(key, default)

    def __init__(__self__, *,
                 client_id: str,
                 principal_id: str):
        pulumi.set(__self__, "client_id", client_id)
        pulumi.set(__self__, "principal_id", principal_id)

    @property
    @pulumi.getter(name="clientId")
    def client_id(self) -> str:
        return pulumi.get(self, "client_id")

    @property
    @pulumi.getter(name="principalId")
    def principal_id(self) -> str:
        return pulumi.get(self, "principal_id")


@pulumi.output_type
class IdentityPropertiesResponse(dict):
    def __getitem__(self, key: str) -> Any:
        return super().__getitem__(key)

    def get(self, key: str, default = None) -> Any:
        return super().get(key, default)

    def __init__(__self__, *,
                 user_assigned_identities: Optional[Mapping[str, UserAssignedIdentityResponse]] = None):
        if user_assigned_identities is not None:
            pulumi.set(__self__, "user_assigned_identities", user_assigned_identities)

    @property
    @pulumi.getter(name="userAssignedIdentities")
    def user_assigned_identities(self) -> Optional[Mapping[str, UserAssignedIdentityResponse]]:
        return pulumi.get(self, "user_assigned_identities")


@pulumi.input_type
class ClusterArgs:
    def __init__(__self__, *,
                 identity: Optional[pulumi.Input[IdentityPropertiesArgs]] = None):
        if identity is not None:
            pulumi.set(__self__, "identity", identity)

    @property
    @pulumi.getter
    def identity(self) -> Optional[pulumi.Input[IdentityPropertiesArgs]]:
        return pulumi.get(self, "identity")


class Cluster(pulumi.CustomResource):
    def __init__(__self__,
                 resource_name: str,
                 opts: Optional[pulumi.ResourceOptions] = None,
                 identity: Optional[pulumi.Input[IdentityPropertiesArgs]] = None):
        __props__ = ClusterArgs.__new__(ClusterArgs)
        __props__.__dict__["identity"] = identity
        super(Cluster, __self__).__init__(
            'test:index:Cluster',
            resource_name,
            __props__,
            opts)

    @property
    @pulumi.getter
    def identity(self) -> pulumi.Output[Optional[IdentityPropertiesResponse]]:
        return pulumi.get(self, "identity")


cluster = Cluster("Cluster",
    identity=IdentityPropertiesArgs(
        user_assigned_identities=[
            "foobar",
        ]
    ),
)
