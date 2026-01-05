// Copyright 2025, Pulumi Corporation.  All rights reserved.


import { PolicyPack, ResourceValidationArgs } from "@pulumi/policy";

new PolicyPack("remediate", {
    enforcementLevel: "advisory",
    policies: [
        {
            name: "fixup",
            description: "Sets property to config",
            enforcementLevel: "remediate",
            remediateResource: (args : ResourceValidationArgs) => {
                if (args.type !== "simple:index:Resource") {
                    return undefined;
                }

                let config = args.getConfig<{ value: boolean }>();

                if (config.value != args.props.value) {
                    return {
                        "value": config.value,
                    };
                }

                return undefined;
            },
        },
    ],
});
