# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)


def validate(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return
    
    config = args.get_config()

    if config["value"] != args.props["value"]:
        report_violation("Property was " + str(args.props["value"]).lower())

PolicyPack(
    name="config",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="allowed",
            description="Verifies properties",
            enforcement_level=EnforcementLevel.MANDATORY,
            validate=validate,
        ),
    ],
)
