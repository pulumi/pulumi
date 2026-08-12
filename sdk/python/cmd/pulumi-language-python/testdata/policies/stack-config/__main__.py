# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)

import pulumi

config = pulumi.Config()
value = config.require_bool("value")

def validate(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return

    if args.props["value"] != value:
        report_violation("Property was " + str(args.props["value"]).lower())

PolicyPack(
    name="stack-config",
    enforcement_level=EnforcementLevel.MANDATORY,
    policies=[
        ResourceValidationPolicy(
            name="validate-" + str(value).lower(),
            description="Verifies property is " + str(value).lower(),
            enforcement_level=EnforcementLevel.MANDATORY,
            validate=validate,
        ),
    ],
)
