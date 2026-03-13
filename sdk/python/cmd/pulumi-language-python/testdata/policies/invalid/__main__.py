# Copyright 2026, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)

def fail(args, report_violation):
    raise Exception("Should never run.")

PolicyPack(
    name="invalid",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="all",
            description="Invalid policy name",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=fail,
        ),
    ],
)
