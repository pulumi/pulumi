# Copyright 2024, Pulumi Corporation.  All rights reserved.

import pulumi
from pulumi.dynamic import Resource

from provider import SimpleProvider


class SimpleResource(Resource):
    authenticated: pulumi.Output[str]
    color: pulumi.Output[str]

    def __init__(self, name):
        super().__init__(SimpleProvider(), name, {"authenticated": None, "color": None})


r = SimpleResource("foo")
pulumi.export("authenticated", r.authenticated)
pulumi.export("color", r.color)
