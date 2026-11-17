# Copyright 2026, Pulumi Corporation.  All rights reserved.

import pulumi_random as random
from pulumi import automation as auto

PROJECT_NAME = "stale-parameterized-packageref"


def preview(stack_name, program):
    stack = auto.create_or_select_stack(
        stack_name=stack_name,
        project_name=PROJECT_NAME,
        program=program,
    )
    try:
        stack.preview()
    finally:
        try:
            stack.workspace.remove_stack(stack_name)
        except Exception:
            pass


preview("stack-1", lambda: random.Password("my-password", length=16))
print("First preview succeeded")

preview("stack-2", lambda: random.Uuid("my-uuid"))
print("Second preview succeeded")
