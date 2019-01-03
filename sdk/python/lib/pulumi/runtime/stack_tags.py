# Copyright 2016-2019, Pulumi Corporation.
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
Runtime support for Pulumi stack tags.  Please use pulumi.get_stack_tag or pulumi.get_stack_tags instead.
"""
from typing import Dict

import json
import os

def get_stack_tags() -> Dict[str, str]:
    """
    Returns the stack tags from the environment.
    """
    if 'PULUMI_STACK_TAGS' in os.environ:
        env_tags = os.environ['PULUMI_STACK_TAGS']
        return json.loads(env_tags)
    return dict()

def get_stack_tag(n: str) -> str:
    """
    Returns a stack tag value or None if it is unset.
    """
    env_dict = get_stack_tags()
    if env_dict is not None and n in list(env_dict.keys()):
        return env_dict[n]

    return None
