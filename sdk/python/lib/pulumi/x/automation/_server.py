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

import asyncio
import grpc
import sys
import traceback
from contextlib import suppress

from ._workspace import PulumiFn
from ... import log
from ...runtime.proto import language_pb2, plugin_pb2, LanguageRuntimeServicer
from ...runtime import run_in_stack, reset_options, set_all_config
from ...errors import RunError

_py_version_less_than_3_7 = sys.version_info[0] == 3 and sys.version_info[1] < 7


class LanguageServer(LanguageRuntimeServicer):
    program: PulumiFn

    def __init__(self, program: PulumiFn) -> None:
        self.program = program  # type: ignore

    @staticmethod
    def on_pulumi_exit():
        # Reset globals
        reset_options()

    def GetRequiredPlugins(self, request, context):
        return language_pb2.GetRequiredPluginsResponse()

    def Run(self, request, context):
        # Configure the runtime so that the user program hooks up to Pulumi as appropriate.
        engine_address = request.args[0] if request.args else ""
        reset_options(
            project=request.project,
            monitor_address=request.monitor_address,
            engine_address=engine_address,
            stack=request.stack,
            parallel=request.parallel,
            preview=request.dryRun
        )

        if request.config:
            set_all_config(request.config)

        # The strategy here is derived from sdk/python/cmd/pulumi-language-python-exec
        result = language_pb2.RunResponse()
        loop = asyncio.new_event_loop()

        try:
            loop.run_until_complete(run_in_stack(self.program))
        except RunError as exn:
            msg = str(exn)
            log.error(msg)
            result.error = str(msg)
            return result
        except grpc.RpcError as exn:
            # If the monitor is unavailable, it is in the process of shutting down or has already
            # shut down. Don't emit an error if this is the case.
            if exn.code() == grpc.StatusCode.UNAVAILABLE:
                log.debug("Resource monitor has terminated, shutting down.")
            else:
                msg = f"RPC error: {exn.details()}"
                log.error(msg)
                result.error = msg
                return result
        except Exception as exn:
            msg = str(f"python inline source runtime error: {exn}\n{traceback.format_exc()}")
            log.error(msg)
            result.error = msg
            return result
        finally:
            # If there's an exception during `run_in_stack`, it may result in pending asyncio tasks remaining unresolved
            # at the time the loop is closed, which results in a `Task was destroyed but it is pending!` error being
            # logged to stdout. To avoid this, we collect all the unresolved tasks in the loop and cancel them before
            # closing the loop.
            pending = asyncio.Task.all_tasks(loop) if _py_version_less_than_3_7 else asyncio.all_tasks(loop)
            log.debug(f"Cancelling {len(pending)} tasks.")
            for task in pending:
                task.cancel()
                with suppress(asyncio.CancelledError):
                    loop.run_until_complete(task)
            loop.close()
            sys.stdout.flush()
            sys.stderr.flush()

        return result

    def GetPluginInfo(self, request, context):
        return plugin_pb2.PluginInfo()
