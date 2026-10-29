# Copyright 2026, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)


# The pulumi_policy runtime automatically reports the policy as not-applicable
# when a validation function reads an unknown value, so we can access the
# property directly and rely on that lifting for unknown and secret-unknown
# inputs. Secret-known inputs are unwrapped and surfaced as their underlying
# value.
def validate_report(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return

    if args.props["value"]:
        report_violation("value is true")
    else:
        report_violation("value is false")


PolicyPack(
    name="unknowns",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="report",
            description="Reports what the analyzer saw",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_report,
        ),
    ],
)
