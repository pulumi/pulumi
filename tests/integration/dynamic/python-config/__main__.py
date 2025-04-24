# Copyright 2024, Pulumi Corporation.  All rights reserved.

import pulumi
from pulumi.dynamic import (
    CreateResult,
    ResourceProvider,
    ConfigureRequest,
    Config,
    Resource,
)


class SimpleProvider(ResourceProvider):
    password: str
    color: str

    def configure(self, req: ConfigureRequest):
        self.password = req.config.get("password")
        self.color = req.config.get("colors:banana")

    def create(self, props):
        # This simulates using this as a credential to talk to an external system.
        return CreateResult(
            "0",
            {
                "authenticated": "200" if self.password == "s3cret" else "401",
                "color": self.color,
            },
        )


class SimpleResource(Resource):
    authenticated: pulumi.Output[str]
    color: pulumi.Output[str]

    def __init__(self, name):
        super().__init__(SimpleProvider(), name, {"authenticated": None, "color": None})


r = SimpleResource("foo")
pulumi.export("authenticated", r.authenticated)
pulumi.export("color", r.color)
