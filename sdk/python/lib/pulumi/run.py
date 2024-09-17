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

import shlex
import sys
import subprocess
from typing import (
    Awaitable,
    Optional,
    Callable,
)
from concurrent import futures
import grpc
from .automation._server import LanguageServer
from .runtime.settings import _GRPC_CHANNEL_OPTIONS
from .runtime.proto import language_pb2_grpc
from .runtime.settings import SETTINGS
from .runtime.sync_await import _sync_await


def run(
    f: Callable[[], Optional[Awaitable[None]]], argv: Optional[list[str]] = None
) -> None:
    """
    Runs a Pulumi program, if using this function you can run the program with the Pulumi CLI or directly via
    Python.

    :param argv: an optional list of command line arguments, typically from `sys.argv`. Only used when running
        with Python, not the Pulumi CLI.
    :param f: an async function that runs a Pulumi program.
    """

    # Check if we have a monitor attached, start a language server and tell the user to connect to it.
    if not SETTINGS.monitor:
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

        if argv is None:
            argv = sys.argv[1:]
        else:
            argv = argv.copy()

        arg = f"--client=127.0.0.1:{port}"
        # splice arg in before any -- arguments
        argv.insert(0, "pulumi")
        if "--" in argv:
            idx = argv.index("--")
            argv.insert(idx, arg)
        else:
            argv.append(arg)

        check = subprocess.run(argv, check=False)
        if check.returncode != 0:
            cmd = shlex.join(argv)
            raise Exception(f"Command {cmd} failed with exit code {check.returncode}")
    else:
        awaitable = f()
        if awaitable is not None:
            # If we're in this case the event loop is already running, so we can just sync await
            _sync_await(awaitable)
