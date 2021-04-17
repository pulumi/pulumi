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

from typing import Optional, Any, Dict


class StackSettingsSecureConfigValue:
    """ A secret Stack config entry."""
    secure: str

    def __init__(self, secure: str):
        self.secure = secure


class StackSettings:
    """A description of the Stack's configuration and encryption metadata."""
    secrets_provider: Optional[str]
    encrypted_key: Optional[str]
    encryption_salt: Optional[str]
    config: Optional[Dict[str, Any]]

    def __init__(self,
                 secrets_provider: Optional[str] = None,
                 encrypted_key: Optional[str] = None,
                 encryption_salt: Optional[str] = None,
                 config: Optional[Dict[str, Any]] = None):
        self.secrets_provider = secrets_provider
        self.encrypted_key = encrypted_key
        self.encryption_salt = encryption_salt
        self.config = config

    @classmethod
    def _deserialize(cls, data: dict):
        config = data.get("config")
        if config is not None:
            stack_config: Dict[str, Any] = {}
            for key, val in config.items():
                if isinstance(val, str):
                    stack_config[key] = val
                elif "secure" in val:
                    stack_config[key] = StackSettingsSecureConfigValue(**val)
            config = stack_config
        return cls(secrets_provider=data.get("secretsprovider"),
                   encrypted_key=data.get("encryptedkey"),
                   encryption_salt=data.get("encryptionsalt"),
                   config=config)

    def _serialize(self):
        serializable = {}

        # Only add the keys that are present to avoid writing nulls to the
        # Pulumi.[stack].yaml file.
        if self.secrets_provider:
            serializable["secretsprovider"] = self.secrets_provider
        if self.encrypted_key:
            serializable["encryptedkey"] = self.encrypted_key
        if self.encryption_salt:
            serializable["encryptionsalt"] = self.encryption_salt
        if self.config:
            config = {}
            for key, val in self.config.items():
                if isinstance(val, StackSettingsSecureConfigValue):
                    config[key] = {"secure": val.secure}
                else:
                    config[key] = val
            serializable["config"] = config

        return serializable
