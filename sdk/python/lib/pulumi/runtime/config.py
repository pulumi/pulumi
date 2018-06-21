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
from __future__ import absolute_import

import json
import os

# default to an empty map for config.
CONFIG = dict()

def set_config(k, v):
    """
    Sets a configuration variable.  Meant for internal use only.
    """
    CONFIG[k] = v

def get_config_env():
    """
    Returns the environment map that will be used for config checking when variables aren't set.
    """
    if 'PULUMI_CONFIG' in os.environ:
        env_config = os.environ['PULUMI_CONFIG']
        return json.loads(env_config)
    return dict()

def get_config_env_key(k):
    """
    Returns a scrubbed environment variable key, PULUMI_CONFIG_<k>, that can be used for
    setting explicit varaibles.  This is unlike PULUMI_CONFIG which is just a JSON-serialized bag.
    """
    env_key = ''
    for c in k:
        if c == '_' or (c >= 'A' and c <= 'Z') or (c >= '0' and c <= '9'):
            env_key += c
        elif c >= 'a' and c <= 'z':
            env_key += c.upper()
        else:
            env_key += '_'
    return 'PULUMI_CONFIG_%s' % env_key

def get_config(k):
    """
    Returns a configuration variable's value or None if it is unset.
    """
    # If the config has been set explicitly, use it.
    if k in list(CONFIG.keys()):
        return CONFIG[k]

    # If there is a specific PULUMI_CONFIG_<k> environment variable, use it.
    env_key = get_config_env_key(k)
    if env_key in os.environ:
        return os.environ[env_key]

    # If the config hasn't been set, but there is a process-wide PULUMI_CONFIG environment variable, use it.
    env_dict = get_config_env()
    if env_dict is not None and k in list(env_dict.keys()):
        return env_dict[k]

    return None
