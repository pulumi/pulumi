# Copyright 2026, Pulumi Corporation.  All rights reserved.

import pulumi
from pulumi import (
    CustomResource,
    log,
    ErrorHook,
    ErrorHookArgs,
    ResourceHookBinding,
    ResourceOptions,
)


class FlakyCreate(CustomResource):
    """Custom resource that uses testprovider:index:FlakyCreate (fails first create, succeeds on retry)."""

    def __init__(self, name: str, opts: pulumi.ResourceOptions | None = None) -> None:
        super().__init__("testprovider:index:FlakyCreate", name, {}, opts)


def on_error(args: ErrorHookArgs) -> bool:
    log.info(f"onError was called for {args.name} ({args.failed_operation})")

    assert args.name == "res", f"expected name to be 'res', got {args.name!r}"
    assert args.type == "testprovider:index:FlakyCreate", (
        f"expected type to be 'testprovider:index:FlakyCreate', got {args.type!r}"
    )
    assert args.failed_operation == "create", (
        f"expected failed operation 'create', got {args.failed_operation!r}"
    )
    assert len(args.errors) > 0, "expected at least one error message"

    return True


hook = ErrorHook("onError", on_error)

res = FlakyCreate(
    "res",
    ResourceOptions(hooks=ResourceHookBinding(on_error=[hook])),
)
