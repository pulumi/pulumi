# Copyright 2025, Pulumi Corporation.
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

from typing import Optional, Set, Dict, List
from abc import ABC
from enum import Enum

import pulumi

from .property_value import PropertyValue


def _extract_type(urn: str) -> str:
    """
    Helper method to extract the type from a URN.
    """
    parts = urn.split("::")
    return parts[-2] if len(parts) > 1 else ""


def _extract_name(urn: str) -> str:
    """
    Helper method to extract the name from a URN.
    """
    parts = urn.split("::")
    return parts[-1] if len(parts) > 1 else ""


class Parameters:
    """Base class for parameters."""

    pass


class ParametersArgs(Parameters):
    """A parameter value represented as an array of strings."""

    def __init__(self, args: List[str]) -> None:
        self.args = args


class ParametersValue(Parameters):
    """A parameter value represented by an arbitrary array of bytes with a name and version."""

    def __init__(self, name: str, version: str, value: bytes) -> None:
        self.name = name
        self.version = version
        self.value = value


class ParameterizeRequest:
    """Represents a parameterization request."""

    def __init__(self, parameters: Parameters) -> None:
        self.parameters = parameters


class ParameterizeResponse:
    """Represents a parameterization response."""

    def __init__(self, name: str, version: str) -> None:
        self.name = name
        self.version = version


class CheckRequest:
    """
    Represents a request to check the configuration of a resource.
    """

    def __init__(
        self,
        urn: str,
        old_inputs: Dict[str, PropertyValue],
        new_inputs: Dict[str, PropertyValue],
        random_seed: bytes,
    ) -> None:
        """
        :param urn: The unique resource name (URN) of the resource.
        :param old_inputs: The previous inputs of the resource.
        :param new_inputs: The new inputs of the resource.
        :param random_seed: A random seed for the request.
        """
        self.urn = urn
        self.old_inputs = old_inputs
        self.new_inputs = new_inputs
        self.random_seed = random_seed

    @property
    def type(self) -> str:
        """
        Extracts the type from the URN.
        """
        return _extract_type(self.urn)

    @property
    def name(self) -> str:
        """
        Extracts the name from the URN.
        """
        return _extract_name(self.urn)


class CheckFailure:
    """Represents a single failure in a check operation."""

    def __init__(self, property: str, reason: str) -> None:
        self.property = property
        self.reason = reason


class CheckResponse:
    """Represents the response of a check operation."""

    def __init__(
        self,
        inputs: Optional[Dict[str, PropertyValue]] = None,
        failures: Optional[List[CheckFailure]] = None,
    ) -> None:
        self.inputs = inputs
        self.failures = failures


class DiffRequest:
    """
    Represents a request to compute the difference between the old and new states of a resource.
    """

    def __init__(
        self,
        urn: str,
        resource_id: str,
        old_state: Dict[str, PropertyValue],
        new_inputs: Dict[str, PropertyValue],
        ignore_changes: List[str],
    ) -> None:
        """
        :param urn: The unique resource name (URN) of the resource.
        :param resource_id: The ID of the resource.
        :param old_state: The previous state of the resource.
        :param new_inputs: The new inputs for the resource.
        :param ignore_changes: A list of properties to ignore when computing the difference.
        """
        self.urn = urn
        self.resource_id = resource_id
        self.old_state = old_state
        self.new_inputs = new_inputs
        self.ignore_changes = ignore_changes

    @property
    def type(self) -> str:
        """
        Extracts the type from the URN.
        """
        return _extract_type(self.urn)

    @property
    def name(self) -> str:
        """
        Extracts the name from the URN.
        """
        return _extract_name(self.urn)


class PropertyDiffKind(Enum):
    """
    Represents the kind of difference for a property.
    """

    ADD = 0
    ADD_REPLACE = 1
    DELETE = 2
    DELETE_REPLACE = 3
    UPDATE = 4
    UPDATE_REPLACE = 5


class PropertyDiff:
    """
    Represents a difference in a property.
    """

    def __init__(self, kind: PropertyDiffKind, input_diff: bool) -> None:
        """
        :param kind: The kind of difference.
        :param input_diff: Whether the difference is in the input.
        """
        self.kind = kind
        self.input_diff = input_diff


class DiffResponse:
    """
    Represents the response of a diff operation.
    """

    def __init__(
        self,
        changes: Optional[bool] = None,
        replaces: Optional[List[str]] = None,
        stables: Optional[List[str]] = None,
        delete_before_replace: bool = False,
        diffs: Optional[List[str]] = None,
        detailed_diff: Optional[Dict[str, PropertyDiff]] = None,
    ) -> None:
        """
        :param changes: Whether there are changes.
        :param replaces: List of properties that require replacement.
        :param stables: List of stable properties.
        :param delete_before_replace: Whether to delete before replacement.
        :param diffs: List of properties that have differences.
        :param detailed_diff: Detailed differences for properties.
        """
        self.changes = changes
        self.replaces = replaces or []
        self.stables = stables or []
        self.delete_before_replace = delete_before_replace
        self.diffs = diffs or []
        self.detailed_diff = detailed_diff or {}


class InvokeRequest:
    """
    Represents a request to invoke a provider function.
    """

    def __init__(self, tok: str, args: Dict[str, PropertyValue]) -> None:
        """
        :param tok: The token identifying the function to invoke.
        :param args: The arguments for the function.
        """
        self.tok = tok
        self.args = args


class InvokeResponse:
    """
    Represents the response from invoking a provider function.
    """

    def __init__(
        self,
        return_value: Optional[Dict[str, PropertyValue]] = None,
        failures: Optional[List[CheckFailure]] = None,
    ) -> None:
        """
        :param return_value: The return value of the function, if any.
        :param failures: A list of failures, if any occurred.
        """
        self.return_value = return_value
        self.failures = failures or []


class GetSchemaRequest:
    """
    Represents a request to retrieve the schema of a provider.
    """

    def __init__(
        self,
        version: int,
        subpackage_name: Optional[str] = None,
        subpackage_version: Optional[str] = None,
    ) -> None:
        """
        :param version: The version of the schema.
        :param subpackage_name: The name of the sub-package, if any.
        :param subpackage_version: The version of the sub-package, if any.
        """
        self.version = version
        self.subpackage_name = subpackage_name
        self.subpackage_version = subpackage_version


class GetSchemaResponse:
    """
    Represents the response containing the schema of a provider.
    """

    def __init__(self, schema: Optional[str] = None) -> None:
        """
        :param schema: The schema as a string, if available.
        """
        self.schema = schema


class ConfigureRequest:
    """
    Represents a request to configure the provider.
    """

    def __init__(
        self,
        variables: Dict[str, str],
        args: Dict[str, PropertyValue],
        accept_secrets: bool,
        accept_resources: bool,
    ) -> None:
        """
        :param variables: A dictionary of configuration variables.
        :param args: A dictionary of arguments.
        :param accept_secrets: Whether the provider accepts secrets.
        :param accept_resources: Whether the provider accepts resources.
        """
        self.variables = variables
        self.args = args
        self.accept_secrets = accept_secrets
        self.accept_resources = accept_resources


class ConfigureResponse:
    """
    Represents the response to a configure request.
    """

    def __init__(
        self,
        accept_secrets: bool = False,
        supports_preview: bool = False,
        accept_resources: bool = False,
        accept_outputs: bool = False,
    ) -> None:
        """
        :param accept_secrets: Whether the provider accepts secrets.
        :param supports_preview: Whether the provider supports preview.
        :param accept_resources: Whether the provider accepts resources.
        :param accept_outputs: Whether the provider accepts outputs.
        """
        self.accept_secrets = accept_secrets
        self.supports_preview = supports_preview
        self.accept_resources = accept_resources
        self.accept_outputs = accept_outputs


class CreateRequest:
    """
    Represents a request to create a resource.
    """

    def __init__(
        self,
        urn: str,
        properties: Dict[str, PropertyValue],
        timeout: float,
        preview: bool,
    ) -> None:
        """
        :param urn: The unique resource name (URN) of the resource.
        :param properties: The properties of the resource to create.
        :param timeout: The timeout for the creation operation, in seconds.
        :param preview: Whether this is a preview operation.
        """
        self.urn = urn
        self.properties = properties
        self.timeout = timeout
        self.preview = preview

    @property
    def type(self) -> str:
        """
        Extracts the type from the URN.
        """
        return _extract_type(self.urn)

    @property
    def name(self) -> str:
        """
        Extracts the name from the URN.
        """
        return _extract_name(self.urn)


class CreateResponse:
    """
    Represents the response to a create request.
    """

    def __init__(
        self,
        resource_id: Optional[str] = None,
        properties: Optional[Dict[str, PropertyValue]] = None,
    ) -> None:
        """
        :param resource_id: The ID of the created resource.
        :param properties: The properties of the created resource.
        """
        self.resource_id = resource_id
        self.properties = properties or {}


class ReadRequest:
    """
    Represents a request to read the state of a resource.
    """

    def __init__(
        self,
        urn: str,
        resource_id: str,
        properties: Dict[str, PropertyValue],
        inputs: Dict[str, PropertyValue],
    ) -> None:
        """
        :param urn: The unique resource name (URN) of the resource.
        :param resource_id: The ID of the resource.
        :param properties: The properties of the resource.
        :param inputs: The inputs of the resource.
        """
        self.urn = urn
        self.resource_id = resource_id
        self.properties = properties
        self.inputs = inputs

    @property
    def type(self) -> str:
        """
        Extracts the type from the URN.
        """
        return _extract_type(self.urn)

    @property
    def name(self) -> str:
        """
        Extracts the name from the URN.
        """
        return _extract_name(self.urn)


class ReadResponse:
    """
    Represents the response to a read request.
    """

    def __init__(
        self,
        resource_id: Optional[str] = None,
        properties: Optional[Dict[str, PropertyValue]] = None,
        inputs: Optional[Dict[str, PropertyValue]] = None,
    ) -> None:
        """
        :param resource_id: The ID of the resource.
        :param properties: The properties of the resource.
        :param inputs: The inputs of the resource.
        """
        self.resource_id = resource_id
        self.properties = properties or {}
        self.inputs = inputs or {}


class UpdateRequest:
    """
    Represents a request to update a resource.
    """

    def __init__(
        self,
        urn: str,
        resource_id: str,
        olds: Dict[str, PropertyValue],
        news: Dict[str, PropertyValue],
        timeout: float,
        ignore_changes: List[str],
        preview: bool,
    ) -> None:
        """
        :param urn: The unique resource name (URN) of the resource.
        :param resource_id: The ID of the resource.
        :param olds: The old properties of the resource.
        :param news: The new properties of the resource.
        :param timeout: The timeout for the update operation, in seconds.
        :param ignore_changes: A list of properties to ignore during the update.
        :param preview: Whether this is a preview operation.
        """
        self.urn = urn
        self.resource_id = resource_id
        self.olds = olds
        self.news = news
        self.timeout = timeout
        self.ignore_changes = ignore_changes
        self.preview = preview

    @property
    def type(self) -> str:
        """
        Extracts the type from the URN.
        """
        return _extract_type(self.urn)

    @property
    def name(self) -> str:
        """
        Extracts the name from the URN.
        """
        return _extract_name(self.urn)


class UpdateResponse:
    """
    Represents the response to an update request.
    """

    def __init__(self, properties: Optional[Dict[str, PropertyValue]] = None) -> None:
        """
        :param properties: The updated properties of the resource.
        """
        self.properties = properties or {}


class DeleteRequest:
    """
    Represents a request to delete a resource.
    """

    def __init__(
        self,
        urn: str,
        resource_id: str,
        properties: Dict[str, PropertyValue],
        timeout: float,
    ) -> None:
        """
        :param urn: The unique resource name (URN) of the resource.
        :param resource_id: The ID of the resource.
        :param properties: The properties of the resource to delete.
        :param timeout: The timeout for the delete operation, in seconds.
        """
        self.urn = urn
        self.resource_id = resource_id
        self.properties = properties
        self.timeout = timeout

    @property
    def type(self) -> str:
        """
        Extracts the type from the URN.
        """
        return _extract_type(self.urn)

    @property
    def name(self) -> str:
        """
        Extracts the name from the URN.
        """
        return _extract_name(self.urn)


class ConstructRequest:
    """
    Represents a request to construct a new resource.
    """

    def __init__(
        self,
        resource_type: str,
        name: str,
        inputs: Dict[str, PropertyValue],
        options: pulumi.ResourceOptions,
    ) -> None:
        """
        :param resource_type: The type of the resource to construct.
        :param name: The name of the resource.
        :param inputs: The input properties for the resource.
        :param options: The options for the resource.
        """
        self.resource_type = resource_type
        self.name = name
        self.inputs = inputs
        self.options = options


class ConstructResponse:
    """
    Represents the response to a construct request.
    """

    def __init__(
        self,
        urn: str,
        state: Dict[str, PropertyValue],
        state_dependencies: Dict[str, Set[str]],
    ) -> None:
        """
        :param urn: The URN of the constructed resource.
        :param state: The state of the constructed resource.
        :param state_dependencies: The dependencies of the resource's state.
        """
        self.urn = urn
        self.state = state
        self.state_dependencies = state_dependencies


class CallRequest:
    """
    Represents a request to call a provider function.
    """

    def __init__(
        self,
        tok: str,
        args: Dict[str, PropertyValue],
    ) -> None:
        """
        :param tok: The token identifying the function to call.
        :param args: The arguments for the function.
        """
        self.tok = tok
        self.args = args


class CallResponse:
    """
    Represents the response to a call request.
    """

    def __init__(
        self,
        return_value: Optional[Dict[str, PropertyValue]] = None,
        return_dependencies: Optional[Dict[str, Set[str]]] = None,
        failures: Optional[List[CheckFailure]] = None,
    ) -> None:
        """
        :param return_value: The return value of the function, if any.
        :param return_dependencies: The dependencies of the return value.
        :param failures: A list of failures, if any occurred.
        """
        self.return_value = return_value or {}
        self.return_dependencies = return_dependencies or {}
        self.failures = failures or []


class Provider(ABC):
    """
    Abstract base class for a Pulumi provider.
    """

    async def parameterize(self, request: ParameterizeRequest) -> ParameterizeResponse:
        """
        Handle the parameterize request.
        """
        raise NotImplementedError("The method 'parameterize' is not implemented")

    async def get_schema(self, request: GetSchemaRequest) -> GetSchemaResponse:
        """
        Handle the get_schema request.
        """
        raise NotImplementedError("The method 'get_schema' is not implemented")

    async def check_config(self, request: CheckRequest) -> CheckResponse:
        """
        Handle the check_config request.
        """
        return CheckResponse(inputs=request.new_inputs)

    async def diff_config(self, request: DiffRequest) -> DiffResponse:
        """
        Handle the diff_config request.
        """
        return DiffResponse()

    async def configure(self, request: ConfigureRequest) -> ConfigureResponse:
        """
        Handle the configure request.
        """
        return ConfigureResponse(
            accept_secrets=True,
            supports_preview=True,
            accept_outputs=True,
            accept_resources=True,
        )

    async def invoke(self, request: InvokeRequest) -> InvokeResponse:
        """
        Handle the invoke request.
        """
        raise NotImplementedError("The method 'invoke' is not implemented")

    async def create(self, request: CreateRequest) -> CreateResponse:
        """
        Handle the create request.
        """
        raise NotImplementedError("The method 'create' is not implemented")

    async def read(self, request: ReadRequest) -> ReadResponse:
        """
        Handle the read request.
        """
        return ReadResponse(
            resource_id=request.resource_id,
            properties=request.properties,
            inputs=request.inputs,
        )

    async def check(self, request: CheckRequest) -> CheckResponse:
        """
        Handle the check request.
        """
        return CheckResponse(inputs=request.new_inputs)

    async def diff(self, request: DiffRequest) -> DiffResponse:
        """
        Handle the diff request.
        """
        return DiffResponse()

    async def update(self, request: UpdateRequest) -> UpdateResponse:
        """
        Handle the update request.
        """
        raise NotImplementedError("The method 'update' is not implemented")

    async def delete(self, request: DeleteRequest) -> None:
        """
        Handle the delete request.
        """
        raise NotImplementedError("The method 'delete' is not implemented")

    async def construct(self, request: ConstructRequest) -> ConstructResponse:
        """
        Handle the construct request.
        """
        raise NotImplementedError("The method 'construct' is not implemented")

    async def call(self, request: CallRequest) -> CallResponse:
        """
        Handle the call request.
        """
        raise NotImplementedError("The method 'call' is not implemented")
