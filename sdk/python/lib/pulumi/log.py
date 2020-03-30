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

"""
Utility functions for logging messages to the diagnostic stream of the Pulumi CLI.
"""
import asyncio
import sys
from typing import Optional, TYPE_CHECKING

from .runtime.settings import get_engine
from .runtime.proto import engine_pb2

if TYPE_CHECKING:
    from .resource import Resource


def debug(msg: str, resource: Optional['Resource'] = None, stream_id: Optional[int] = None, ephemeral: Optional[bool] = None) -> None:
    """
    Logs a message to the Pulumi CLI's debug channel, associating it with a resource
    and stream_id if provided.

    :param str msg: The message to send to the Pulumi CLI.
    :param Optional[Resource] resource: If provided, associate this message with the given resource in the Pulumi CLI.
    :param Optional[int] stream_id: If provided, associate this message with a stream of other messages.
    """
    engine = get_engine()
    if engine is not None:
        _log(engine, engine_pb2.DEBUG, msg, resource, stream_id, ephemeral)
    else:
        print("debug: " + msg, file=sys.stderr)


def info(msg: str, resource: Optional['Resource'] = None, stream_id: Optional[int] = None, ephemeral: Optional[bool] = None) -> None:
    """
    Logs a message to the Pulumi CLI's info channel, associating it with a resource
    and stream_id if provided.

    :param str msg: The message to send to the Pulumi CLI.
    :param Optional[Resource] resource: If provided, associate this message with the given resource in the Pulumi CLI.
    :param Optional[int] stream_id: If provided, associate this message with a stream of other messages.
    """
    engine = get_engine()
    if engine is not None:
        _log(engine, engine_pb2.INFO, msg, resource, stream_id, ephemeral)
    else:
        print("info: " + msg, file=sys.stderr)


def warn(msg: str, resource: Optional['Resource'] = None, stream_id: Optional[int] = None, ephemeral: Optional[bool] = None) -> None:
    """
    Logs a message to the Pulumi CLI's warning channel, associating it with a resource
    and stream_id if provided.

    :param str msg: The message to send to the Pulumi CLI.
    :param Optional[Resource] resource: If provided, associate this message with the given resource in the Pulumi CLI.
    :param Optional[int] stream_id: If provided, associate this message with a stream of other messages.
    """
    engine = get_engine()
    if engine is not None:
        _log(engine, engine_pb2.WARNING, msg, resource, stream_id, ephemeral)
    else:
        print("warning: " + msg, file=sys.stderr)


def error(msg: str, resource: Optional['Resource'] = None, stream_id: Optional[int] = None, ephemeral: Optional[bool] = None):
    """
    Logs a message to the Pulumi CLI's error channel, associating it with a resource
    and stream_id if provided.

    :param str msg: The message to send to the Pulumi CLI.
    :param Optional[Resource] resource: If provided, associate this message with the given resource in the Pulumi CLI.
    :param Optional[int] stream_id: If provided, associate this message with a stream of other messages.
    """
    engine = get_engine()
    if engine is not None:
        _log(engine, engine_pb2.ERROR, msg, resource, stream_id, ephemeral)
    else:
        print("error: " + msg, file=sys.stderr)


def _log(engine, severity, message, resource, stream_id, ephemeral):
    if stream_id is None:
        stream_id = 0

    # If we can log synchronously, do so. The worst thing we can do with a log message is exit
    # before we have the chance to send the message.
    #
    # We can log synchronously as long as we haven't been given a resource to attach to. If we have,
    # we have to asynchronously resolve the URN first.
    async def do_log():
        resolved_urn = await resource.urn.future()
        req = engine_pb2.LogRequest(severity=severity, message=message, urn=resolved_urn,
                                    streamId=stream_id, ephemeral=ephemeral)
        engine.Log(req)

    if resource is not None:
        asyncio.ensure_future(do_log())
    else:
        req = engine_pb2.LogRequest(severity=severity, message=message, urn="",
                                    streamId=stream_id, ephemeral=ephemeral)
        engine.Log(req)
