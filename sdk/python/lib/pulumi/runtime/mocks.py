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
Mocks for testing.
"""
import asyncio
import logging
from abc import ABC, abstractmethod
from typing import Optional, Awaitable, Tuple, Union, Any, TYPE_CHECKING

import grpc
from google.protobuf import empty_pb2
from . import rpc
from .settings import Settings, configure, get_stack, get_project
from .sync_await import _sync_await
from ..runtime.proto import engine_pb2, engine_pb2_grpc, provider_pb2, resource_pb2, resource_pb2_grpc
from ..runtime.stack import run_pulumi_func
from ..output import Output

if TYPE_CHECKING:
    from ..resource import Resource


loop = None


def test(fn):
    def wrapper(*args, **kwargs):
        asyncio.set_event_loop(loop)
        _sync_await(run_pulumi_func(lambda: _sync_await(Output.from_input(fn(*args, **kwargs)).future())))
    return wrapper


class Mocks(ABC):
    @abstractmethod
    def call(self, token: str, args: dict, provider: Optional[str]) -> dict:
        return {}

    @abstractmethod
    def new_resource(self, type_: str, name: str, inputs: dict, provider: Optional[str], id_: Optional[str]) -> Tuple[str, dict]:
        return ("", {})


class MockMonitor:
    mocks: Mocks

    def __init__(self, mocks: Mocks):
        self.mocks = mocks

    def make_urn(self, parent: str, type_: str, name: str) -> str:
        if parent != "":
            qualifiedType = parent.split("::")[2]
            parentType = qualifiedType.split("$").pop()
            type_ = parentType + "$" + type_

        return "urn:pulumi:" + "::".join([get_stack(), get_project(), type_, name])

    def Invoke(self, request):
        args = rpc.deserialize_properties(request.args)

        ret = self.mocks.call(request.tok, args, request.provider)

        asyncio.set_event_loop(loop)
        ret_proto = _sync_await(asyncio.ensure_future(rpc.serialize_properties(ret, {})))

        fields = {"failures": None, "return": ret_proto}
        return provider_pb2.InvokeResponse(**fields)

    def ReadResource(self, request):
        state = rpc.deserialize_properties(request.properties)

        _, state = self.mocks.new_resource(request.type, request.name, state, request.provider, request.id)

        asyncio.set_event_loop(loop)
        props_proto = _sync_await(asyncio.ensure_future(rpc.serialize_properties(state, {})))

        urn = self.make_urn(request.parent, request.type, request.name)
        return resource_pb2.ReadResourceResponse(urn=urn, properties=props_proto)

    def RegisterResource(self, request):
        inputs = rpc.deserialize_properties(request.object)

        id_, state = self.mocks.new_resource(request.type, request.name, inputs, request.provider, request.importId)

        asyncio.set_event_loop(loop)
        obj_proto = _sync_await(rpc.serialize_properties(state, {}))

        urn = self.make_urn(request.parent, request.type, request.name)
        return resource_pb2.RegisterResourceResponse(urn=urn, id=id_, object=obj_proto)

    def RegisterResourceOutputs(self, request):
        #pylint: disable=unused-argument
        return empty_pb2.Empty()


class MockEngine:
    logger: logging.Logger

    def __init__(self, logger: Optional[logging.Logger]):
        self.logger = logger if logger is not None else logging.getLogger()

    def Log(self, request):
        if request.severity == engine_pb2.DEBUG:
            self.logger.debug(request.message)
        elif request.severity == engine_pb2.INFO:
            self.logger.info(request.message)
        elif request.severity == engine_pb2.WARNING:
            self.logger.warning(request.message)
        elif request.severity == engine_pb2.ERROR:
            self.logger.error(request.message)


def set_mocks(mocks: Mocks,
              project: Optional[str] = None,
              stack: Optional[str] = None,
              preview: Optional[bool] = None,
              logger: Optional[logging.Logger] = None):
    """
    set_mocks configures the Pulumi runtime to use the given mock data for testing.
    """
    settings = Settings(monitor=MockMonitor(mocks),
                        engine=MockEngine(logger),
                        project=project if project is not None else 'project',
                        stack=stack if stack is not None else 'stack',
                        dry_run=preview,
                        test_mode_enabled=True)
    configure(settings)

    # Make sure we have an event loop.
    global loop
    loop = asyncio.get_event_loop()
