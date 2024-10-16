# Copyright 2024, Pulumi Corporation.
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

from typing import Optional, overload

import pulumi
from pulumi.metadata import get_project


class Config:
    """
    Config is a wrapper around :class:`pulumi.Config` that can be used in
    dynamic providers. This class does not provide methods to retrieve secrets
    in the form of `:class:`pulumi.Output`. Since these methods are called at
    runtime, secrets will not be serialized as part of Pulumi's dynamic provider
    implementation, and it is safe to use the plain get and require methods
    instead. Do note however that you should not log or print the values.
    """

    name: str
    """
    The configuration bag's logical name that uniquely identifies it. The
    default is the name of the current project.
    """

    _config: pulumi.Config

    def __init__(self, name: Optional[str] = None):
        if not name:
            name = get_project()
        if not isinstance(name, str):
            raise TypeError("Expected name to be a string")
        self.name = name
        self._config = pulumi.Config(name)

        # Note: don't try to be too fancy here with loops or inspection,
        # manually setting the methods like this allows mypy/pyright to
        # understand the types correctly.
        self.get = self._config.get
        self.get_bool = self._config.get_bool
        self.get_int = self._config.get_int
        self.get_float = self._config.get_float
        self.get_object = self._config.get_object
        self.require = self._config.require
        self.require_bool = self._config.require_bool
        self.require_int = self._config.require_int
        self.require_float = self._config.require_float
        self.require_object = self._config.require_object
