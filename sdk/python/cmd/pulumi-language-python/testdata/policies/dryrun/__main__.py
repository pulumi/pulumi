# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)
import pulumi

def validate_truthiness(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return

    if pulumi.runtime.is_dry_run():
        if not args.props["value"]:
            report_violation("This is a test error")

PolicyPack(
    name="dryrun",
    enforcement_level=EnforcementLevel.MANDATORY,
    policies=[
        ResourceValidationPolicy(
            name="dry",
            description="Verifies properties are true on dryrun",
            enforcement_level=EnforcementLevel.MANDATORY,
            validate=validate_truthiness,
        ),
    ],
)
