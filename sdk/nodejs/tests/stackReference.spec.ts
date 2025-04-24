// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as assert from "assert";
import * as pulumi from "..";

class TestMocks implements pulumi.runtime.Mocks {
    outputs: any;

    constructor(outputs: any) {
        this.outputs = outputs;
    }

    call(args: pulumi.runtime.MockCallArgs): Record<string, any> {
        throw new Error(`unknown function ${args.token}`);
    }

    newResource(args: pulumi.runtime.MockResourceArgs): { id: string | undefined; state: Record<string, any> } {
        switch (args.type) {
            case "pulumi:pulumi:StackReference":
                return {
                    id: `${args.name}_id`,
                    state: {
                        outputs: this.outputs,
                    },
                };
            default:
                throw new Error(`unknown type ${args.type}`);
        }
    }
}

describe("StackReference.getOutputDetails", () => {
    // The two tests don't share a mock because in the JS SDK,
    // if a map item is a secret, the entire map gets promoted to secret.

    it("supports plain text", async () => {
        pulumi.runtime.setMocks(
            new TestMocks({
                bucket: "mybucket-1234",
            }),
        );
        const ref = new pulumi.StackReference("foo");

        assert.deepStrictEqual(await ref.getOutputDetails("bucket"), {
            value: "mybucket-1234",
        });
    });

    it("supports secrets", async () => {
        pulumi.runtime.setMocks(
            new TestMocks({
                password: pulumi.secret("supersecretpassword"),
            }),
        );
        const ref = new pulumi.StackReference("foo");

        assert.deepStrictEqual(await ref.getOutputDetails("password"), {
            secretValue: "supersecretpassword",
        });
    });

    it("types applies correctly", async () => {
        const passwordValue = "supersecretpassword";
        pulumi.runtime.setMocks(
            new TestMocks({
                password: pulumi.secret(passwordValue),
            }),
        );
        const ref = new pulumi.StackReference("foo");
        const password: pulumi.Output<number> = ref.outputs["password"].apply((x) => x.length);

        assert.deepStrictEqual(await password.promise(), passwordValue.length);
    });
});
