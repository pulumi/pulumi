// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

import { PolicyPack, validateResourceOfType } from "@pulumi/policy";
import * as random from "@pulumi/random";

new PolicyPack("validate-resource-test-policy", {
    policies: [
        {
            name: "dynamic-no-state-with-value-1",
            description: "Prohibits setting state to 1 on dynamic resources.",
            enforcementLevel: "mandatory",
            validateResource: (args, reportViolation) => {
                if (args.type === "pulumi-nodejs:dynamic:Resource") {
                    if (args.props.state === 1) {
                        reportViolation("'state' must not have the value 1.")
                    }
                }
            },
        },
        // More than one policy.
        {
            name: "dynamic-no-state-with-value-2",
            description: "Prohibits setting state to 2 on dynamic resources.",
            enforcementLevel: "mandatory",
            validateResource: (args, reportViolation) => {
                if (args.type === "pulumi-nodejs:dynamic:Resource") {
                    if (args.props.state === 2) {
                        reportViolation("'state' must not have the value 2.")
                    }
                }
            },
        },
        // Multiple validateResource callbacks.
        {
            name: "dynamic-no-state-with-value-3-or-4",
            description: "Prohibits setting state to 3 or 4 on dynamic resources.",
            enforcementLevel: "mandatory",
            validateResource: [
                (args, reportViolation) => {
                    if (args.type === "pulumi-nodejs:dynamic:Resource") {
                        if (args.props.state === 3) {
                            reportViolation("'state' must not have the value 3.")
                        }
                    }
                },
                (args, reportViolation) => {
                    if (args.type === "pulumi-nodejs:dynamic:Resource") {
                        if (args.props.state === 4) {
                            reportViolation("'state' must not have the value 4.")
                        }
                    }
                },
            ],
        },
        // Strongly-typed.
        {
            name: "randomuuid-no-keepers",
            description: "Prohibits creating a RandomUuid without any 'keepers'.",
            enforcementLevel: "mandatory",
            validateResource: validateResourceOfType(random.RandomUuid, (it, args, reportViolation) => {
                if (!it.keepers || Object.keys(it.keepers).length === 0) {
                    reportViolation("RandomUuid must not have an empty 'keepers'.")
                }
            }),
        },
        // Specifying a URN explicitly has no affect for validateResource.
        {
            name: "dynamic-no-state-with-value-5",
            description: "Prohibits setting state to 5 on dynamic resources.",
            enforcementLevel: "mandatory",
            validateResource: (args, reportViolation) => {
                if (args.type === "pulumi-nodejs:dynamic:Resource") {
                    if (args.props.state === 5) {
                        reportViolation("'state' must not have the value 5.", "some-urn")
                    }
                }
            },
        },
        // Validate other type checks work as expected.
        {
            name: "test-type-checks",
            description: "Policy used to test type checks.",
            enforcementLevel: "mandatory",
            validateResource: (args, reportViolation) => {
                if (args.type !== "random:index/randomPassword:RandomPassword") {
                    return;
                }
                if (!args.isType(random.RandomPassword)) {
                    throw new Error("`isType` did not return the expected value.");
                }
                const randomPassword = args.asType(random.RandomPassword);
                if (!randomPassword) {
                    throw new Error("`asType` did not return the expected value.");
                }
                if (randomPassword.length !== 42) {
                    throw new Error("`randomPassword.length` did not return the expected value.");
                }
            },
        },
        // Ensure that the resource can contain large strings.
        {
            name: "large-resource",
            description: "Ensures that large string properties are set properly.",
            enforcementLevel: "mandatory",
            validateResource: (args) => {
                if (args.type === "pulumi-nodejs:dynamic:Resource") {
                    if (args.props.state === 6) {
                        const longString = "a".repeat(5 * 1024 * 1024);
                        const expected = longString.length;
                        const result = args.props.longString.length;
                        if (result !== expected) {
                            throw new Error(`'longString' had expected length of ${expected}, got ${result}`);
                        }
                    }
                }
            },
        }
    ],
});
