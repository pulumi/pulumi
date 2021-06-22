# Copyright 2016-2018, Pulumi Corporation.
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

"""
Runtime support for the Pulumi configuration system.  Please use pulumi.Config instead.
"""
from typing import Dict, Any, List, Optional, Set

import json
import os

# default to an empty map for config.
CONFIG: Dict[str, Any] = dict()

# default to an empty set for config secret keys.
_SECRET_KEYS: Set[str] = set()


def set_config(k: str, v: Any):
    """
    Sets a configuration variable.  Meant for internal use only.
    """
    CONFIG[k] = v


def set_all_config(config: Dict[str, str], secret_keys: Optional[List[str]] = None) -> None:
    """
    Overwrites the config map and optional list of secret keys.
    """
    new_config = {}
    for key, value in config.items():
        new_config[key] = value
    global CONFIG
    CONFIG = new_config

    if secret_keys is not None:
        global _SECRET_KEYS
        _SECRET_KEYS = set(secret_keys)


def get_config_env() -> Dict[str, Any]:
    """
    Returns the environment map that will be used for config checking when variables aren't set.
    """
    if 'PULUMI_CONFIG' in os.environ:
        env_config = os.environ['PULUMI_CONFIG']
        return json.loads(env_config)
    return dict()


def get_config_env_key(k: str) -> str:
    """
    Returns a scrubbed environment variable key, PULUMI_CONFIG_<k>, that can be used for
    setting explicit variables.  This is unlike PULUMI_CONFIG which is just a JSON-serialized bag.
    """
    env_key = ''
    for c in k:
        if c == '_' or 'A' <= c <= 'Z' or '0' <= c <= '9':
            env_key += c
        elif 'a' <= c <= 'z':
            env_key += c.upper()
        else:
            env_key += '_'
    return 'PULUMI_CONFIG_%s' % env_key


def get_config_secret_keys_env() -> List[str]:
    """
    Returns the list of config keys that contain secrets.
    """
    if 'PULUMI_CONFIG_SECRET_KEYS' in os.environ:
        keys = os.environ['PULUMI_CONFIG_SECRET_KEYS']
        return json.loads(keys)
    return []


def get_config(k: str) -> Any:
    """
    Returns a configuration variable's value or None if it is unset.
    """
    # If the config has been set explicitly, use it.
    if k in CONFIG:
        return CONFIG[k]

    # If there is a specific PULUMI_CONFIG_<k> environment variable, use it.
    env_key = get_config_env_key(k)
    if env_key in os.environ:
        return os.environ[env_key]

    # If the config hasn't been set, but there is a process-wide PULUMI_CONFIG environment variable, use it.
    env_dict = get_config_env()
    if env_dict is not None and k in env_dict:
        return env_dict[k]

    return None


def is_config_secret(k: str) -> bool:
    """
    Returns True if the configuration variable is a secret.
    """
    return k in _SECRET_KEYS
