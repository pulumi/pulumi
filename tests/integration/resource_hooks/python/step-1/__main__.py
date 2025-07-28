# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi import (
    log,
    ResourceHook,
    ResourceHookArgs,
    ResourceHookBinding,
    ResourceOptions,
)
from random_ import Random, Component


def before_create(args: ResourceHookArgs) -> None:
    log.info(f"before_create was called with length = {args.new_inputs.get('length')}")
    assert args.name == "res", f"Expected name 'res', got {args.name}"
    assert args.type == "testprovider:index:Random", (
        f"Expected type 'testprovider:index:Random', got {args.type}"
    )


before_create_hook = ResourceHook("before_create", before_create)


def before_delete(args: ResourceHookArgs) -> None:
    log.info(f"before_delete was called with length = {args.old_inputs.get('length')}")
    assert args.name == "res", f"Expected name 'res', got {args.name}"
    assert args.type == "testprovider:index:Random", (
        f"Expected type 'testprovider:index:Random', got {args.type}"
    )


before_delete_hook = ResourceHook("before_delete", before_create)


res = Random(
    "res",
    length=10,
    opts=ResourceOptions(
        hooks=ResourceHookBinding(
            before_create=[before_create_hook],
            before_delete=[before_delete_hook],
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
