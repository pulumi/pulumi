# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi import (
    log,
    ResourceHook,
    ResourceHookArgs,
    ResourceHookBinding,
    ResourceOptions,
    ResourceTransformArgs,
    ResourceTransformResult,
)
from random_ import Random, Component


def fun(args: ResourceHookArgs) -> None:
    if args.name == "res":
        log.info(f"fun was called with length = {args.new_inputs.get('length')}")
        assert args.type == "testprovider:index:Random", (
            f"Expected type 'testprovider:index:Random', got {args.type}"
        )
    elif args.name == "comp":
        childId = args.new_outputs.get("childId")
        log.info(f"fun_comp was called with child = {childId}")
        if not childId:
            raise ValueError(f"expected non empty childId, got '{childId}'")
        assert args.type == "testprovider:index:Component", (
            f"Expected type 'testprovider:index:Component', got {args.type}"
        )
    elif args.name == "child-comp":
        log.info(f"fun_child was called with length = {args.new_inputs.get('length')}")
        assert args.type == "testprovider:index:Random", (
            f"Expected type 'testprovider:index:Random', got {args.type}"
        )
    else:
        raise Exception(f"got unexpected component name: {args.name}")


hook = ResourceHook("hook_fun", fun)


def transform(args: ResourceTransformArgs) -> ResourceTransformResult:
    opts = args.opts
    opts.hooks = ResourceHookBinding.merge(
        opts.hooks,
        ResourceHookBinding(
            after_create=[fun],
        ),
    )
    return ResourceTransformResult(
        props=args.props,
        opts=opts,
    )


res = Random(
    "res",
    length=10,
    opts=ResourceOptions(
        transforms=[transform],
    ),
)


comp = Component(
    "comp",
    length=7,
    opts=ResourceOptions(
        transforms=[transform],
    ),
)
