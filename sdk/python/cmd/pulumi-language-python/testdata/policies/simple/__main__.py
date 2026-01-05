# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)


def validate_truthiness(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return

    if args.props["value"]:
        report_violation("This is a test warning")

def validate_falsiness(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return

    if not args.props["value"]:
        report_violation("This is a test error")

PolicyPack(
    name="simple",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="truthiness",
            description="Verifies properties are true",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_truthiness,
        ),
        ResourceValidationPolicy(
            name="falsiness",
            description="Verifies properties are false",
            enforcement_level=EnforcementLevel.MANDATORY,
            validate=validate_falsiness,
        ),
    ],
)
