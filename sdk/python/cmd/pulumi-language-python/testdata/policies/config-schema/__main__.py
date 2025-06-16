# Copyright 2025, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
    PolicyConfigSchema,
)


def validate(args, report_violation):
    if args.resource_type != "simple:index:Resource":
        return

    config = args.get_config()

    if config["value"] != args.props["value"]:
        report_violation("Property was " + str(args.props["value"]).lower())

PolicyPack(
    name="config-schema",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="validator",
            description="Verifies property matches config",
            enforcement_level=EnforcementLevel.ADVISORY,
            config_schema=PolicyConfigSchema(
                properties={
                    "value": {
                        "type": "boolean",
                    },
                    "names": {
                        "type": "array",
                        "items": {
                            "type": "string",
                        },
                        "minItems": 1,
                    },
                },
                required=["value", "names"],
            ),
            validate=validate,
        ),
    ],
)
