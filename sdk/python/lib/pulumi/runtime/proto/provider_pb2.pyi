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


class ConstructResponse:

    def __init__(self,
                 urn: Optional[str]=None,
                 state: Optional[Struct]=None,
                 stateDependencies: Optional[Dict[str,PropertyDependencies]]=None) -> void:
        pass

    class PropertyDependencies:
        urns: List[str]

        def __init__(self, urns: List[str]) -> void:
            pass

    urn: str
    state: Struct
    stateDependencies: Dict[str,PropertyDependencies]


class CheckResponse:
    def __init__(self, inputs: Optional[Struct]=None, failures: List[CheckFailure]=[]) -> void:
        pass

    inputs: Struct
    failures: List[CheckFailure]


class CheckFailure:
    property: str
    reason: str


class ConfigureResponse:

    def __init__(self,
                 acceptSecrets: bool=False,
                 supportsPreview: bool=False,
                 acceptResources: bool=False) -> void: ...

    acceptSecrets: bool
    supportsPreview: bool
    acceptResources: bool
