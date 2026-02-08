# Copyright 2025, Pulumi Corporation.  All rights reserved.

import json
from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)


def validate(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return

    tag = args.stack_tags["value"]
    if tag is None:
        report_violation("Stack tag 'value' is required")
        return

    expected = json.loads(tag)

    if expected != args.props["value"]:
        report_violation("Property was " + str(args.props["value"]).lower())

PolicyPack(
    name="stack-tags",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="allowed",
            description="Verifies property equals the stack tag value",
            enforcement_level=EnforcementLevel.MANDATORY,
            validate=validate,
        ),
    ],
)
