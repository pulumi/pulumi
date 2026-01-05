// Copyright 2025, Pulumi Corporation.  All rights reserved.


import { PolicyPack, ResourceValidationArgs } from "@pulumi/policy";

new PolicyPack("config-schema", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "validator",
            description: "Verifies property matches config",
            enforcementLevel: "advisory",
            configSchema: {
                properties: {
                    "value": {
                        type: "boolean",
                    },
                    "names": {
                        type: "array",
                        items: {
                            type: "string",
                        },
                        minItems: 1,
                    },
                },
                required: ["value", "names"],
            },
            validateResource: (args : ResourceValidationArgs, reportViolation) => {
                if (args.type !== "simple:index:Resource") {
                    return;
                }

                let config = args.getConfig<{ value: boolean, names: string[] }>();

                if (config.names.includes(args.name)) {
                    if (args.props.value !== config.value) {
                        reportViolation("Property was " + args.props.value);
                    }
                }
            },
        },
    ],
});
