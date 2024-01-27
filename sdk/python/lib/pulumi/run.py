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
import subprocess
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
from .automation._server import LanguageServer
import grpc
from concurrent import futures
from .runtime.settings import _GRPC_CHANNEL_OPTIONS
from .runtime.proto import language_pb2_grpc

if TYPE_CHECKING:
    from .output import Input, Inputs
    from .runtime.stack import Stack


def run(f: Callable[[], Optional[Awaitable[None]]]) -> None:
    """
    Runs a Pulumi program.

    :param f: an async function that runs a Pulumi program.
    """
    loop = asyncio.get_event_loop()

    # Check if we have a monitor attached, start a language serveer and tell the user to connect to it.
    if not os.environ.get("PULUMI_MONITOR"):
        server = grpc.server(
            futures.ThreadPoolExecutor(
                max_workers=4
            ),  # pylint: disable=consider-using-with
            options=_GRPC_CHANNEL_OPTIONS,
        )
        language_server = LanguageServer(f)
        language_pb2_grpc.add_LanguageRuntimeServicer_to_server(language_server, server)

        port = server.add_insecure_port(address="127.0.0.1:0")
        server.start()

        cmd = os.environ.get("PULUMI_DEBUG_COMMAMD")
        arg = f"--client=127.0.0.1:{port}"
        if not cmd:
            print(f"Connect via `pulumi {arg}`")
            input("Press Enter to exit...")
        else:
            subprocess.run(cmd.split(" ") + [arg])
    else:
        awaitable = f()
        if awaitable is not None:
            loop.run_until_complete(awaitable)
