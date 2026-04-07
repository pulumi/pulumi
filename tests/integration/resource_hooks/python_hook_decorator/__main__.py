# Copyright 2025, Pulumi Corporation.  All rights reserved.

from random_ import Random

import pulumi.runtime
from pulumi import (
    log,
    ResourceHook,
    ResourceHookOptions,
    ResourceHookArgs,
    ResourceHookBinding,
    ResourceOptions,
)

#@ResourceHook("before_create", ResourceHookOptions(on_dry_run=True))
async def test_hook(args: ResourceHookArgs):
    log.info(f"hook called {pulumi.runtime.is_dry_run()}")

test_hook = ResourceHook("before_create", test_hook, ResourceHookOptions(on_dry_run=True))


res = Random(
    "res",
    length=10,
    opts=ResourceOptions(
        hooks=ResourceHookBinding(before_create=[test_hook])
    ),
)