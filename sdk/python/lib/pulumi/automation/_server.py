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
import logging
import sys
import traceback
import contextvars
from contextlib import suppress

import grpc

from .. import log
from ..errors import RunError
from ..runtime import reset_options, run_in_stack, set_all_config
from ..runtime.proto import LanguageRuntimeServicer, language_pb2, plugin_pb2
from ._workspace import PulumiFn


class LanguageServer(LanguageRuntimeServicer):
    program: PulumiFn

    def __init__(self, program: PulumiFn) -> None:
        self.program = program

    def GetRequiredPlugins(self, request, context):
        return language_pb2.GetRequiredPluginsResponse()

    def _exception_handler(self, loop, context):
        # Exception are normally handler deeper in the stack. If this class of
        # exception bubble up to here, something is wrong and we should stop
        # the event loop
        if "exception" in context and isinstance(context["exception"], grpc.RpcError):
            loop.stop()
        else:
            loop.default_exception_handler(context)

    def Run(self, request, context):
        _suppress_unobserved_task_logging()

        def run():
            # Configure the runtime so that the user program hooks up to Pulumi as appropriate.
            engine_address = request.args[0] if request.args else ""
            organization = (
                request.organization if request.organization else "organization"
            )
            reset_options(
                project=request.project,
                monitor_address=request.monitor_address,
                engine_address=engine_address,
                stack=request.stack,
                parallel=request.parallel,
                preview=request.dryRun,
                organization=organization,
            )

            if request.config:
                secret_keys = (
                    request.configSecretKeys if request.configSecretKeys else None
                )
                set_all_config(request.config, secret_keys)

            # The strategy here is derived from sdk/python/cmd/pulumi-language-python-exec
            result = language_pb2.RunResponse()
            loop = asyncio.new_event_loop()

            loop.set_exception_handler(self._exception_handler)
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
                # pylint: disable=no-member
                if exn.code() == grpc.StatusCode.UNAVAILABLE:
                    log.debug("Resource monitor has terminated, shutting down.")
                else:
                    msg = f"RPC error: {exn.details()}"
                    log.error(msg)
                    result.error = msg
                    return result
            except Exception as exn:
                msg = str(
                    f"python inline source runtime error: {exn}\n{traceback.format_exc()}"
                )
                log.error(msg)
                result.error = msg
                return result
            finally:
                # If there's an exception during `run_in_stack`, it may result in pending asyncio tasks remaining unresolved
                # at the time the loop is closed, which results in a `Task was destroyed but it is pending!` error being
                # logged to stdout. To avoid this, we collect all the unresolved tasks in the loop and cancel them before
                # closing the loop.
                pending = (
                    # lint safety: we use the python version here to track deprecations
                    asyncio.all_tasks(loop)
                )  # pylint: disable=no-member
                log.debug(f"Cancelling {len(pending)} tasks.")
                for task in pending:
                    task.cancel()
                    with suppress(asyncio.CancelledError):
                        loop.run_until_complete(task)
                loop.close()
                sys.stdout.flush()
                sys.stderr.flush()

            return result

        ctx = contextvars.copy_context()
        return ctx.run(run)

    def GetPluginInfo(self, request, context):
        return plugin_pb2.PluginInfo()


def _suppress_unobserved_task_logging():
    """Suppresses logs about faulted unobserved tasks. This is similar to
    Python Pulumi user programs. See rationale in
    `sdk/python/cmd/pulumi-language-python-exec`.

    """
    logging.getLogger("asyncio").setLevel(logging.CRITICAL)
