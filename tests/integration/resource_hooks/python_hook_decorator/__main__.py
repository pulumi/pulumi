# Copyright 2025, Pulumi Corporation.  All rights reserved.

from random_ import Random

import pulumi.runtime
from pulumi import (
    log,
    resource_hook,
    ResourceHookOptions,
    ResourceHookArgs,
    ResourceHookBinding,
    ResourceOptions,
)


@resource_hook("after_create")
def after_create(args: ResourceHookArgs) -> None:
    log.info(f"after_create was called with length = {args.new_inputs.get('length')}")
    assert args.name == "res", f"Expected name 'res', got {args.name}"
    assert args.type == "testprovider:index:Random", (
        f"Expected type 'testprovider:index:Random', got {args.type}"
    )

@resource_hook("before_create", ResourceHookOptions(on_dry_run=True))
async def test_hook(args: ResourceHookArgs):
    log.info(f"hook called {pulumi.runtime.is_dry_run()}")


res = Random(
    "res",
    length=10,
    opts=ResourceOptions(
        hooks=ResourceHookBinding(before_create=[test_hook], after_create=[after_create])
    ),
)