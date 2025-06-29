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

from typing import MutableMapping, Optional

_SECRET_SENTINEL = "[secret]"


class ConfigValue:
    """
    ConfigValue is the input/output of a `pulumi config` command.
    It has a plaintext value, and an option boolean indicating secretness.
    """

    value: str
    secret: bool = False

    def __init__(self, value: str, secret: bool = False):
        self.value = value
        self.secret = secret

    def __repr__(self):
        return _SECRET_SENTINEL if self.secret else repr(self.value)


ConfigMap = MutableMapping[str, ConfigValue]
"""ConfigMap is a map of string to ConfigValue."""


class ConfigOptions:
    """
    ConfigOptions is used to configure config operations.

    Args:
        path: When True, treats config keys containing dots as paths in a nested object.

        config_file: Optional path to a specific config file to use instead of the stack's
                     default config file.
        show_secrets: When True, secret values will be shown in plaintext rather than as
                      the value "[secret]".
    """

    path: bool
    config_file: Optional[str]
    show_secrets: bool

    def __init__(
        self,
        path: bool = False,
        config_file: Optional[str] = None,
        show_secrets: bool = False,
    ):
        self.path = path
        self.config_file = config_file
        self.show_secrets = show_secrets


class GetAllConfigOptions:
    """
    GetAllConfigOptions is used to configure get all config operations.

    Args:
        path: When True, treats config keys containing dots as paths in a nested object.
              This requires Pulumi CLI version 3.95 or higher.
        config_file: Optional path to a specific config file to use instead of the stack's
                     default config file.
        show_secrets: When True, secret values will be shown in plaintext rather than as
                      the value "[secret]".
    """

    path: bool
    config_file: Optional[str]
    show_secrets: bool

    def __init__(
        self,
        path: bool = False,
        config_file: Optional[str] = None,
        show_secrets: bool = False,
    ):
        self.path = path
        self.config_file = config_file
        self.show_secrets = show_secrets
