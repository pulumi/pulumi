import * as pulumi from "@pulumi/pulumi";
import "jest";
import { willThrow } from "."

// Mocks are unused, but previously importing and using the pulumi package in
// the tests would break source maps
// https://github.com/pulumi/pulumi/issues/9218
pulumi.runtime.setMocks({
    newResource: (args: pulumi.runtime.MockResourceArgs) => {
        return {
            id: `${args.name}-id`,
            state: args.inputs,
        };
    },
    call: (args: pulumi.runtime.MockCallArgs) => {
        return {};
    },
})

it("a failing test so we can inspect the stacktrace reported by jest", async () => {
    willThrow();
});
