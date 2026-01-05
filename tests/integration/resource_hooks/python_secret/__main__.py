# Copyright 2025, Pulumi Corporation.  All rights reserved.

from echo import Echo
from pulumi import (
    log,
    Output,
    ResourceHookArgs,
    ResourceHookBinding,
    ResourceOptions,
)

async def hook(args: ResourceHookArgs):
    out = args.new_inputs["echo"]
    assert await out.is_secret()
    assert await out.future() == "hello secret"
    log.info("hook called")

Echo(
    "echo",
    echo=Output.secret("hello secret"),
    opts=ResourceOptions(hooks=ResourceHookBinding(before_create=[hook])),
)
