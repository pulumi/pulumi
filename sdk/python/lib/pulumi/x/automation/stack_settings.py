# Copyright 2016-2020, Pulumi Corporation.
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

from dataclasses import dataclass
from typing import Optional, Mapping, Any, Union


@dataclass
class StackSettingsSecureConfigValue:
    """ A secret Stack config entry."""
    secure: str


@dataclass
class StackSettings:
    """A description of the Stack's configuration and encryption metadata."""
    secrets_provider: Optional[str]
    encrypted_key: Optional[str]
    encryption_salt: Optional[str]
    config: Optional[Mapping[str, Any]]

    def __init__(self,
                 secretsProvider: Optional[str] = None,
                 encryptedKey: Optional[str] = None,
                 encryptionSalt: Optional[str] = None,
                 config: Optional[Mapping[str, Any]] = None):
        self.secrets_provider = secretsProvider
        self.encrypted_key = encryptedKey
        self.encryption_salt = encryptionSalt
        if config:
            stack_config = {}
            for key in config:
                val = config[key]
                if type(val) == str:
                    stack_config[key] = val
                elif "secure" in val:
                    stack_config[key] = StackSettingsSecureConfigValue(**val)
            if len(stack_config.keys()) > 0:
                self.config = stack_config
