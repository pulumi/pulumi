# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi import (
    log,
    ResourceHook,
    ResourceHookArgs,
    ResourceHookBinding,
    ResourceOptions,
)
from random_ import Random


def fun(args: ResourceHookArgs) -> None:
    log.info(f"fun was called with length = {args.new_inputs.get('length')}")
    assert args.name == "res1", f"Expected name 'res1', got {args.name}"
    assert args.type == "testprovider:index:Random", (
        f"Expected type 'testprovider:index:Random', got {args.type}"
    )


hook_fun = ResourceHook("hook_fun", fun)


res1 = Random(
    "res1",
    10,
    ResourceOptions(
        hooks=ResourceHookBinding(
            before_create=[hook_fun],
        )
    ),
)
