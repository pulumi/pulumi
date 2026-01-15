# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

"""
Tests that dynamic providers can return inputs from read() method.
This is a regression test for https://github.com/pulumi/pulumi/issues/13839

The issue was that after `pulumi refresh`, `pulumi preview --diff` would show
all properties as changing because the stored inputs didn't match the refreshed
outputs. The fix allows read() to return inputs that will be used for subsequent
diffs.
"""

from pulumi import export
from pulumi.dynamic import Resource, ResourceProvider, CreateResult, ReadResult


class SimpleProvider(ResourceProvider):
    """A simple provider that demonstrates returning inputs from read()."""

    def create(self, props):
        # Create returns the value as both an output
        return CreateResult(id_="test-resource-id", outs={"value": props["value"]})

    def read(self, id_, props):
        # Read returns the current state as both outputs AND inputs.
        # This ensures that after a refresh, subsequent diffs will compare
        # against the refreshed state rather than the original inputs.
        return ReadResult(
            id_=id_,
            outs=props,
            inputs={"value": props["value"]},  # Return inputs to fix diff after refresh
        )


class SimpleResource(Resource):
    def __init__(self, name, value, opts=None):
        super().__init__(
            SimpleProvider(),
            name,
            {"value": value},
            opts,
        )


# Create a simple resource
res = SimpleResource("test", value="hello")

export("resource_id", res.id)
