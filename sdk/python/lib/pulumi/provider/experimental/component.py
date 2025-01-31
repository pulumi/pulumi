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

from dataclasses import dataclass
from enum import Enum
from typing import Optional


class PropertyType(Enum):
    STRING = "string"
    INTEGER = "integer"
    NUMBER = "number"
    BOOLEAN = "boolean"
    OBJECT = "object"


@dataclass
class PropertyDefinition:
    optional: bool = False
    type: Optional[PropertyType] = None
    ref: Optional[str] = None
    description: Optional[str] = None


@dataclass
class TypeDefinition:
    name: str
    type: str
    properties: dict[str, PropertyDefinition]
    properties_mapping: dict[str, str]
    """Mapping from the schema name to the Python name."""
    description: Optional[str] = None


@dataclass
class ComponentDefinition:
    inputs: dict[str, PropertyDefinition]
    outputs: dict[str, PropertyDefinition]
    inputs_mapping: dict[str, str]
    """Mapping from the schema name to the Python name."""
    outputs_mapping: dict[str, str]
    """Mapping from the schema name to the Python name."""
    description: Optional[str] = None
