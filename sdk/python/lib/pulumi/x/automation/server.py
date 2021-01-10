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

import logging
import traceback

from .workspace import PulumiFn
from ...log import *
from ...runtime.proto import language_pb2, plugin_pb2, LanguageRuntimeServicer
from ...runtime import run_in_stack, reset_options, set_all_config
from ...errors import RunError


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

        # Suppress the log output for asyncio (see pulumi-language-python-exec for detailed explanation)
        logging.getLogger("asyncio").setLevel(logging.CRITICAL)

        try:
            loop.run_until_complete(run_in_stack(self.program))
        except RunError as exn:
            result.error = str(exn)
            return result
        except Exception as exn:
            result.error = str(f"python inline source runtime error: {exn}\n{traceback.format_exc()}")
            return result
        finally:
            loop.close()
            sys.stdout.flush()
            sys.stderr.flush()

        return result

    def GetPluginInfo(self, request, context):
        return plugin_pb2.PluginInfo()

