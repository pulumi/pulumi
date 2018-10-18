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

from __future__ import print_function

import sys
from pulumi.runtime import get_engine
from pulumi.runtime.proto import engine_pb2

def debug(msg, resource=None, stream_id=None):
    """
    Logs a message to the Pulumi CLI's debug channel, associating it with a resource
    and stream_id if provided.
    """
    engine = get_engine()
    if engine is not None:
        _log(engine, engine_pb2.DEBUG, msg, resource, stream_id)
    else:
        print("debug: " + msg, file=sys.stderr)

def info(msg, resource=None, stream_id=None):
    """
    Logs a message to the Pulumi CLI's info channel, associating it with a resource
    and stream_id if provided.
    """
    engine = get_engine()
    if engine is not None:
        _log(engine, engine_pb2.INFO, msg, resource, stream_id)
    else:
        print("info: " + msg, file=sys.stderr)

def warn(msg, resource=None, stream_id=None):
    """
    Logs a message to the Pulumi CLI's warning channel, associating it with a resource
    and stream_id if provided.
    """
    engine = get_engine()
    if engine is not None:
        _log(engine, engine_pb2.WARNING, msg, resource, stream_id)
    else:
        print("warning: " + msg, file=sys.stderr)

def error(msg, resource=None, stream_id=None):
    """
    Logs a message to the Pulumi CLI's error channel, associating it with a resource
    and stream_id if provided.
    """
    engine = get_engine()
    if engine is not None:
        _log(engine, engine_pb2.ERROR, msg, resource, stream_id)
    else:
        print("error: " + msg, file=sys.stderr)

def _log(engine, severity, message, resource, stream_id):
    if resource is not None:
        urn = resource.urn
    else:
        urn = ""

    if stream_id is None:
        stream_id = 0

    req = engine_pb2.LogRequest(severity=severity, message=message, urn=urn, streamId=stream_id)
    engine.Log(req)
