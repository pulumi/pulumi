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

from .runtime import settings


def get_organization() -> str:
    """
    Returns the current organization name.
    """
    return settings.get_organization()


def get_project() -> str:
    """
    Returns the current project name.
    """
    return settings.get_project()


def get_stack() -> str:
    """
    Returns the current stack name.
    """
    return settings.get_stack()


def get_root_directory() -> str:
    """
    Returns the current root directory.
    """
    return settings.get_root_directory()
