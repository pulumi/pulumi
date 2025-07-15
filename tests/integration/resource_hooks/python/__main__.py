# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi import (
    log,
    ResourceHook,
    ResourceHookArgs,
    ResourceHookBinding,
    ResourceOptions,
)
from random_ import Random, Component


def fun(args: ResourceHookArgs) -> None:
    log.info(f"fun was called with length = {args.new_inputs.get('length')}")
    assert args.name == "res", f"Expected name 'res', got {args.name}"
    assert args.type == "testprovider:index:Random", (
        f"Expected type 'testprovider:index:Random', got {args.type}"
    )


hook = ResourceHook("hook_fun", fun)


res = Random(
    "res",
    length=10,
    opts=ResourceOptions(
        hooks=ResourceHookBinding(
            before_create=[hook],
        )
    ),
)


def fun_comp(args: ResourceHookArgs) -> None:
    childId = args.new_outputs.get("childId")
    log.info(f"fun_comp was called with child = {childId}")
    if not childId:
        raise ValueError(f"expected non empty childId, got '{childId}'")
    assert args.name == "comp", f"Expected name 'comp', got {args.name}"
    assert args.type == "testprovider:index:Component", (
        f"Expected type 'testprovider:index:Component', got {args.type}"
    )


hook_comp = ResourceHook("hook_fun_comp", fun_comp)


comp = Component(
    "comp",
    length=7,
    opts=ResourceOptions(hooks=ResourceHookBinding(after_create=[hook_comp])),
)
