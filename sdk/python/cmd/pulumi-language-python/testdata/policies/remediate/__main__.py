# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
)


def remediate(args):
    if args.resource_type != "simple:index:Resource":
        return

    config = args.get_config()

    if config["value"] != args.props["value"]:
        return { "value": config["value"] }

PolicyPack(
    name="remediate",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="fixup",
            description="Sets property to config",
            enforcement_level=EnforcementLevel.REMEDIATE,
            remediate=remediate,
        ),
    ],
)
