# Copyright 2024-2025, Pulumi Corporation.

from typing import Any

from pulumi import Output, export, get_organization, get_project, get_stack
from pulumi.dynamic import Resource, ResourceProvider, CreateResult


class MyProvider(ResourceProvider):
    def create(self, inputs):
        return CreateResult(id_="foo", outs={})


class MyResource(Resource):
    def __init__(self, name: str, props: Any):
         super().__init__(MyProvider(), name, props, None)


MyResource("my-resource", {
    "inputs": [Output.secret(Output.from_input(i)) for i in range(1, 50)]
})


for i in range(1, 50):
    export(f"echo-{i}", Output.secret(Output.from_input(i)))


export("organization", get_organization())
export("project", get_project())
export("stack", get_stack())
