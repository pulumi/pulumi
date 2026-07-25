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

from pulumi.provider.experimental.analyzer import (
    ComponentDefinition,
    PropertyDefinition,
    PropertyType,
)
from pulumi.provider.experimental.schema import generate_schema


def test_generate_schema_with_namespace():
    schema = generate_schema("name", "1.2.3", "namespace", {}, {}, [])
    assert schema.name == "name"
    assert schema.namespace == "namespace"


def test_generate_schema_no_namespace():
    schema = generate_schema("name", "1.2.3", None, {}, {}, [])
    assert schema.name == "name"
    assert schema.namespace is None


def _component(name, *, inputs=None, outputs=None, extends=None, abstract=False):
    return ComponentDefinition(
        name=name,
        module="m",
        inputs=inputs or {},
        inputs_mapping={},
        outputs=outputs or {},
        outputs_mapping={},
        extends=extends,
        abstract=abstract,
    )


def test_generate_schema_local_extends_no_required_features():
    components = {
        "Base": _component(
            "Base", outputs={"baseOutput": PropertyDefinition(type=PropertyType.STRING)}
        ),
        "Derived": _component(
            "Derived",
            outputs={"baseOutput": PropertyDefinition(type=PropertyType.STRING)},
            extends="#/resources/name:index:Base",
        ),
    }
    schema = generate_schema("name", "1.2.3", None, components, {}, [])
    resource = schema.resources["name:index:Derived"].to_json()
    assert resource["extends"] == {"$ref": "#/resources/name:index:Base"}
    # A locally flattened schema carries the full member set, so it must not require the inheritance feature.
    assert schema.required_features is None
    assert "requiredFeatures" not in schema.to_json()


def test_generate_schema_external_extends_requires_inheritance():
    components = {
        "MyService": _component(
            "MyService",
            outputs={"endpoint": PropertyDefinition(type=PropertyType.STRING)},
            extends="/basecomp/v1.0.0/schema.json#/resources/basecomp:index:Service",
        ),
    }
    schema = generate_schema("name", "1.2.3", None, components, {}, [])
    resource = schema.resources["name:index:MyService"].to_json()
    assert resource["extends"] == {
        "$ref": "/basecomp/v1.0.0/schema.json#/resources/basecomp:index:Service"
    }
    # A sparse schema relies on the binder to materialize inherited members, so it must declare the feature.
    assert schema.required_features == ["inheritance"]
    assert schema.to_json()["requiredFeatures"] == ["inheritance"]


def test_generate_schema_abstract_component():
    components = {"MyComponent": _component("MyComponent", abstract=True)}
    schema = generate_schema("name", "1.2.3", None, components, {}, [])
    assert schema.resources["name:index:MyComponent"].to_json()["abstract"] is True
