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


"""Manually constructed mypy typings. We should explore automated
mypy typing generation from protobufs in the future.

"""

from typing import Dict, List, Optional
from google.protobuf.struct_pb2 import Struct


class ConstructRequest:
    class PropertyDependencies:
        urns: List[str]

    project: str
    stack: str
    config: Dict[str,str]
    dryRun: bool
    parallel: int
    monitorEndpoint: str
    type: str
    name: str
    parent: str
    inputs: Struct
    inputDependencies: Dict[str,PropertyDependencies]
    protect: bool
    providers: Dict[str, str]
    aliases: List[str]
    dependencies: List[str]
    configSecretKeys: List[str]


class ConstructResponse:

    def __init__(self,
                 urn: Optional[str]=None,
                 state: Optional[Struct]=None,
                 stateDependencies: Optional[Dict[str,PropertyDependencies]]=None) -> None:
        pass

    class PropertyDependencies:
        urns: List[str]

        def __init__(self, urns: List[str]) -> None:
            pass

    urn: str
    state: Struct
    stateDependencies: Dict[str,PropertyDependencies]


class CallRequest:
    class ArgumentDependencies:
        urns: List[str]

    tok: str
    args: Struct
    argDependencies: Dict[str,ArgumentDependencies]
    provider: str
    version: str
    project: str
    stack: str
    config: Dict[str,str]
    configSecretKeys: List[str]
    dryRun: bool
    parallel: int
    monitorEndpoint: str


class CallResponse:

    def __init__(self, **kwargs) -> None:
        pass

    class ReturnDependencies:
        urns: List[str]

        def __init__(self, urns: List[str]) -> None:
            pass


class CheckResponse:
    def __init__(self, inputs: Optional[Struct]=None, failures: List[CheckFailure]=[]) -> None:
        pass

    inputs: Struct
    failures: List[CheckFailure]


class CheckFailure:
    def __init__(self, property: str, reason: str) -> None:
        pass

    property: str
    reason: str


class ConfigureResponse:

    def __init__(self,
                 acceptSecrets: bool=False,
                 supportsPreview: bool=False,
                 acceptResources: bool=False,
                 acceptOutputs: bool=False) -> None: ...

    acceptSecrets: bool
    supportsPreview: bool
    acceptResources: bool
    acceptOutputs: bool

class GetSchemaRequest:
    version: int


class GetSchemaResponse:
    def __init__(self, schema: str):
        pass
