// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";
import { Resource } from "./resource";

const config = new pulumi.Config();
const testScenario = config.requireNumber("scenario");

switch (testScenario) {
    case 1:
        // Don't create any resources.
        break;

    case 2:
        // Create a resource that doesn't violate any policies.
        const hello = new Resource("hello", { hello: "world" });
        break;

    case 3:
        // Violates the first policy.
        const a = new Resource("a", { state: 1 });
        break;

    case 4:
        // Violates the second policy.
        const b = new Resource("b", { state: 2 });
        break;

    case 5:
        // Violates the first validation function of the third policy.
        const c = new Resource("c", { state: 3 });
        break;

    case 6:
        // Violates the second validation function of the third policy.
        const d = new Resource("d", { state: 4 });
        break;

    case 7:
        // Violates the fourth policy.
        const r1 = new random.RandomUuid("r1");
        break;

    case 8:
        // Doesn't violate the fourth policy.
        const r2 = new random.RandomUuid("r2", {
            keepers: {
                foo: "bar",
            },
        });
        break;

    case 9:
        // Violates the fifth policy.
        const e = new Resource("e", { state: 5 });
        break;

    case 10:
        // Create a resource to test the strongly-typed helpers.
        const pass = new random.RandomPassword("pass", {
            length: 42,
        });
        break;

    case 11:
        // Create a resource with a large string property. This passes as specified.
        const longString = "a".repeat(5 * 1024 * 1024);
        const largeStringResource = new Resource("large-resource", { state: 6, longString })
}
