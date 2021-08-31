# Copyright 2016-2021, Pulumi Corporation.
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

# These are copied from pulumi-azure-native or hand-written to
# compensate for an incomplete codegen test setup, we could fix the
# test to code-gen this from schema.

from collections import namedtuple

import pulumi


@pulumi.output_type
class StorageAccountKeyResponse(dict):
    """
    An access key for the storage account.
    """
    def __init__(__self__, *,
                 creation_time: str,
                 key_name: str,
                 permissions: str,
                 value: str):
        """
        An access key for the storage account.
        :param str creation_time: Creation time of the key, in round trip date format.
        :param str key_name: Name of the key.
        :param str permissions: Permissions for the key -- read-only or full permissions.
        :param str value: Base 64-encoded value of the key.
        """

        pulumi.set(__self__, "creation_time", creation_time)
        pulumi.set(__self__, "key_name", key_name)
        pulumi.set(__self__, "permissions", permissions)
        pulumi.set(__self__, "value", value)

    @property
    @pulumi.getter(name="creationTime")
    def creation_time(self) -> str:
        """
        Creation time of the key, in round trip date format.
        """
        return pulumi.get(self, "creation_time")

    @property
    @pulumi.getter(name="keyName")
    def key_name(self) -> str:
        """
        Name of the key.
        """
        return pulumi.get(self, "key_name")

    @property
    @pulumi.getter
    def permissions(self) -> str:
        """
        Permissions for the key -- read-only or full permissions.
        """
        return pulumi.get(self, "permissions")

    @property
    @pulumi.getter
    def value(self) -> str:
        """
        Base 64-encoded value of the key.
        """
        return pulumi.get(self, "value")


CodegenTest = namedtuple('CodegenTest', ['outputs'])
Outputs = namedtuple('Outputs', ['StorageAccountKeyResponse'])

outputs = Outputs(StorageAccountKeyResponse)
codegentest = CodegenTest(outputs)
