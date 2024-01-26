# Copyright 2016-2024, Pulumi Corporation.
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

import asyncio
import copy
import os
import warnings
from typing import (
    Awaitable,
    Optional,
    List,
    Any,
    Mapping,
    Sequence,
    Union,
    Set,
    Callable,
    Tuple,
    TYPE_CHECKING,
    cast,
)
from . import _types
from .metadata import get_project, get_stack
from .runtime import known_types
from .runtime.resource import (
    _pkg_from_type,
    get_resource,
    register_resource,
    register_resource_outputs,
    read_resource,
    collapse_alias_to_urn,
    create_urn as create_urn_internal,
    convert_providers,
)
from .runtime.settings import get_root_resource
from .output import _is_prompt, _map_input, _map2_input, T, Output
from . import urn as urn_util
from . import log

if TYPE_CHECKING:
    from .output import Input, Inputs
    from .runtime.stack import Stack

def run(f : Awaitable[None]) -> None:
    """
    Runs a Pulumi program.

    :param f: an async function that runs a Pulumi program.
    """

    # Check if we have a monitor attached, if not fail fast.
    if not os.environ.get("PULUMI_MONITOR"):
        raise Exception("Pulumi engine not attached; ensure you're running your Pulumi program via the `pulumi` CLI")

    loop = asyncio.get_event_loop()
    loop.run_until_complete(f)