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

import asyncio
import os
from subprocess import Popen, PIPE
import time
from typing import Callable, Any, Dict, List, Optional
import grpc
from .. import ComponentResource, CustomResource, Output, InvokeOptions, ResourceOptions, log, Input, Inputs, Resource
from ..runtime.proto import runtime_pb2, runtime_pb2_grpc
from ..runtime.rpc import deserialize_properties, serialize_properties
from ..runtime.settings import SETTINGS

def spawnServer(library_path: str):
    def setting_to_string(setting: Optional[str]) -> str:
        return "" if setting is None else setting
    proc = Popen(["node", "-e", "require('@pulumi/pulumi/remote/server')"], cwd=library_path, stdout=PIPE, env={
        **os.environ,
        'PULUMI_NODEJS_PROJECT': setting_to_string(SETTINGS.project),
        'PULUMI_NODEJS_STACK': setting_to_string(SETTINGS.stack),
        'PULUMI_NODEJS_DRY_RUN': "true" if SETTINGS.dry_run else "false",
        'PULUMI_NODEJS_QUERY_MODE': "false",
        'PULUMI_NODEJS_MONITOR': setting_to_string(SETTINGS.monitor_addr),
        'PULUMI_NODEJS_ENGINE': setting_to_string(SETTINGS.engine_addr),
        'PULUMI_TEST_MODE': "true" if SETTINGS.test_mode_enabled else "false",
        'PULUMI_ENABLE_LEGACY_APPLY': "false",
        'PULUMI_NODEJS_SYNC': "false",
        'PULUMI_NODEJS_PARALLEL': "true",
    })
    port = proc.stdout.readline().decode()[:-1]
    proc.stdout.close()
    channel = grpc.insecure_channel(f'0.0.0.0:{port}')
    stub = runtime_pb2_grpc.RuntimeStub(channel)
    return stub

stubs: Dict[str, runtime_pb2_grpc.RuntimeStub] = dict()

def get_server(library_path: str) -> runtime_pb2_grpc.RuntimeStub:
    stub = stubs.get(library_path, None)
    if stub is None:
        stub = spawnServer(library_path)
        stubs[library_path] = stub
    return stub

# def resource_options_to_dict(opts: ResourceOptions) -> Inputs:
#     d = vars(opts)
#     d.pop("merge", None)
#     return d

async def construct(
        libraryPath: str,
        resource: str,
        name: str,
        args: Any,
        _opts: ResourceOptions) -> Any:
    property_dependencies_resources: Dict[str, List[Resource]] = {}
    args_struct = await serialize_properties(args, property_dependencies_resources)
    # TODO - support opts serialization
    opts_struct = await serialize_properties({}, property_dependencies_resources)
    req = runtime_pb2.ConstructRequest(
        libraryPath=libraryPath,
        resource=resource,
        name=name,
        args=args_struct,
        opts=opts_struct,
    )
    resp = get_server(libraryPath).Construct(req)
    outs = deserialize_properties(resp.outs)
    return outs

class ProxyComponentResource(ComponentResource):
    """
    Abstract base class for proxies around component resources.
    """
    def __init__(self,
                 t: str,
                 name: str,
                 library_path: str,
                 library_name: str,
                 inputs: Inputs,
                 outputs: Dict[str, None],
                 opts: Optional[ResourceOptions] = None) -> None:
        if opts is None or opts.urn is None:
            async def do_construct():
                r = await construct(library_path, library_name, name, inputs, opts)
                return r["urn"]
            urn = asyncio.ensure_future(do_construct())
            opts = ResourceOptions.merge(opts, ResourceOptions(urn=urn))
        props = {
            **inputs,
            **outputs,
        }
        super().__init__(t, name, props, opts)
