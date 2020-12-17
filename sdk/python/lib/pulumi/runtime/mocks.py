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
from typing import Dict, NamedTuple, Optional, Tuple, TYPE_CHECKING

from google.protobuf import empty_pb2
from . import rpc
from .settings import Settings, configure, get_stack, get_project, get_root_resource
from .sync_await import _sync_await
from ..runtime.proto import engine_pb2, provider_pb2, resource_pb2
from ..runtime.stack import Stack, run_pulumi_func

if TYPE_CHECKING:
    from ..resource import Resource


def test(fn):
    def wrapper(*args, **kwargs):
        from .. import Output  # pylint: disable=import-outside-toplevel
        _sync_await(run_pulumi_func(lambda: _sync_await(Output.from_input(fn(*args, **kwargs)).future())))
    return wrapper

class Mocks(ABC):
    """
    Mocks is an abstract class that allows subclasses to replace operations normally implemented by the Pulumi engine with
    their own implementations. This can be used during testing to ensure that calls to provider functions and resource constructors
    return predictable values.
    """
    @abstractmethod
    def call(self, token: str, args: dict, provider: Optional[str]) -> dict:
        """
        call mocks provider-implemented function calls (e.g. aws.get_availability_zones).

        :param str token: The token that indicates which function is being called. This token is of the form "package:module:function".
        :param dict args: The arguments provided to the function call.
        :param Optional[str] provider: If provided, the identifier of the provider instance being used to make the call.
        """
        return {}

    @abstractmethod
    def new_resource(self, type_: str, name: str, inputs: dict, provider: Optional[str], id_: Optional[str]) -> Tuple[Optional[str], dict]:
        """
        new_resource mocks resource construction calls. This function should return the physical identifier and the output properties
        for the resource being constructed.

        :param str type_: The token that indicates which resource type is being constructed. This token is of the form "package:module:type".
        :param str name: The logical name of the resource instance.
        :param dict inputs: The inputs for the resource.
        :param Optional[str] provider: If provided, the identifier of the provider instnace being used to manage this resource.
        :param Optional[str] id_: If provided, the physical identifier of an existing resource to read or import.
        """
        return ("", {})


class MockMonitor:
    class ResourceRegistration(NamedTuple):
        urn: str
        id: str
        state: dict

    mocks: Mocks
    resources: Dict[str, ResourceRegistration]

    def __init__(self, mocks: Mocks):
        self.mocks = mocks
        self.resources = dict()

    def make_urn(self, parent: str, type_: str, name: str) -> str:
        if parent != "":
            qualifiedType = parent.split("::")[2]
            parentType = qualifiedType.split("$").pop()
            type_ = parentType + "$" + type_

        return "urn:pulumi:" + "::".join([get_stack(), get_project(), type_, name])

    def Invoke(self, request):
        args = rpc.deserialize_properties(request.args)

        if request.tok == "pulumi:pulumi:getResource":
            registered_resource = self.resources.get(args["urn"])
            if registered_resource is None:
                raise Exception(f"unknown resource {args['urn']}")
            ret_proto = _sync_await(rpc.serialize_properties(registered_resource._asdict(), {}))
            fields = {"failures": None, "return": ret_proto}
            return provider_pb2.InvokeResponse(**fields)

        ret = self.mocks.call(request.tok, args, request.provider)

        ret_proto = _sync_await(rpc.serialize_properties(ret, {}))

        fields = {"failures": None, "return": ret_proto}
        return provider_pb2.InvokeResponse(**fields)

    def ReadResource(self, request):
        state = rpc.deserialize_properties(request.properties)

        id_, state = self.mocks.new_resource(request.type, request.name, state, request.provider, request.id)

        props_proto = _sync_await(rpc.serialize_properties(state, {}))

        urn = self.make_urn(request.parent, request.type, request.name)

        self.resources[urn] = MockMonitor.ResourceRegistration(urn, id_, state)

        return resource_pb2.ReadResourceResponse(urn=urn, properties=props_proto)

    def RegisterResource(self, request):
        urn = self.make_urn(request.parent, request.type, request.name)

        if request.type == "pulumi:pulumi:Stack":
            return resource_pb2.RegisterResourceResponse(urn=urn)

        inputs = rpc.deserialize_properties(request.object)

        id_, state = self.mocks.new_resource(request.type, request.name, inputs, request.provider, request.importId)

        obj_proto = _sync_await(rpc.serialize_properties(state, {}))

        self.resources[urn] = MockMonitor.ResourceRegistration(urn, id_, state)

        return resource_pb2.RegisterResourceResponse(urn=urn, id=id_, object=obj_proto)

    def RegisterResourceOutputs(self, request):
        # pylint: disable=unused-argument
        return empty_pb2.Empty()

    def SupportsFeature(self, request):
        # pylint: disable=unused-argument
        return type('SupportsFeatureResponse', (object,), {'hasSupport' : True})


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
    set_mocks configures the Pulumi runtime to use the given mocks for testing.
    """
    settings = Settings(monitor=MockMonitor(mocks),
                        engine=MockEngine(logger),
                        project=project if project is not None else 'project',
                        stack=stack if stack is not None else 'stack',
                        dry_run=preview,
                        test_mode_enabled=True)
    configure(settings)

    # Ensure a new root stack resource has been initialized.
    if get_root_resource() is None:
        Stack(lambda: None)
