# Copyright 2026, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    StackValidationPolicy,
)


def validate_stack(args, report_violation):
    count = sum(1 for r in args.resources if r.resource_type == "simple:index:Resource")
    if count > 1:
        report_violation("Found an extra simple:index:Resource")


PolicyPack(
    name="analyze-stack",
    enforcement_level=EnforcementLevel.MANDATORY,
    policies=[
        StackValidationPolicy(
            name="stack-size",
            description="Stack must contain at most one simple:index:Resource",
            enforcement_level=EnforcementLevel.MANDATORY,
            validate=validate_stack,
        ),
    ],
)
