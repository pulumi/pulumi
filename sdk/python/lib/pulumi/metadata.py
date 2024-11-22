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

from .runtime.settings import get_organization as runtime_go
from .runtime.settings import get_project as runtime_gp
from .runtime.settings import get_stack as runtime_gs


def get_organization() -> str:
    """
    Returns the current organization name.
    """
    return runtime_go()


def get_project() -> str:
    """
    Returns the current project name.
    """
    return runtime_gp()


def get_stack() -> str:
    """
    Returns the current stack name.
    """
    return runtime_gs()
