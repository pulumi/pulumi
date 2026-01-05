# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)


def validate(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return

    if args.props["value"]:
        report_violation("Property was " + str(args.props["value"]).lower())

PolicyPack(
    name="enforcement-config",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="false",
            description="Verifies property is false",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate,
        ),
    ],
)
