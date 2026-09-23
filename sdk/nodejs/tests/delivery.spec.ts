// Copyright 2026, Pulumi Corporation.
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

const dbOutputs = { endpoint: "db.internal:5432", password: pulumi.secret("hunter2") };
const appOutputs = { url: "https://app.example.com" };

class DeliveryMocks implements pulumi.runtime.Mocks {
    public readonly registered: pulumi.runtime.MockResourceArgs[] = [];

    call(args: pulumi.runtime.MockCallArgs): Record<string, any> {
        throw new Error(`unknown function ${args.token}`);
    }

    newResource(args: pulumi.runtime.MockResourceArgs): { id: string | undefined; state: Record<string, any> } {
        this.registered.push(args);
        switch (args.type) {
            case "pulumi:delivery:Stack":
                return {
                    id: `${args.name}_id`,
                    state: { stack: args.inputs.stack, outputs: dbOutputs, secretOutputNames: ["password"] },
                };
            case "pulumi:delivery:StackGroup":
                return {
                    id: `${args.name}_id`,
                    state: {
                        members: [
                            { stack: "org/db/prod", outputs: dbOutputs, secretOutputNames: ["password"] },
                            { stack: "org/app/prod", outputs: appOutputs, secretOutputNames: [] },
                        ],
                    },
                };
            default:
                throw new Error(`unknown type ${args.type}`);
        }
    }
}

describe("delivery", () => {
    let mocks: DeliveryMocks;
    beforeEach(() => {
        mocks = new DeliveryMocks();
        pulumi.runtime.setMocks(mocks);
    });

    it("Stack passes its stack and source through and reads outputs by name", async () => {
        const db = new pulumi.delivery.Stack("db", { stack: "org/db/prod", source: { directory: "infra/db" } });

        assert.strictEqual(await db.stack.promise(), "org/db/prod");
        assert.deepStrictEqual(mocks.registered[0].inputs, {
            stack: "org/db/prod",
            source: { directory: "infra/db" },
        });

        const endpoint = db.getOutput("endpoint");
        assert.strictEqual(await endpoint.promise(), "db.internal:5432");
        assert.strictEqual(await endpoint.isSecret, false);

        const password = db.requireOutput("password");
        assert.strictEqual(await password.promise(), "hunter2");
        assert.strictEqual(await password.isSecret, true);

        assert.strictEqual(await db.getOutput("missing").promise(), undefined);
        await assert.rejects(db.requireOutput("missing").promise(), /Required output 'missing' does not exist/);
    });

    it("StackGroup reads each member's outputs by stack", async () => {
        const group = new pulumi.delivery.StackGroup("services", {
            stacks: [
                { stack: "org/db/prod", source: { directory: "infra/db" } },
                { stack: "org/app/prod", source: { directory: "infra/app" } },
            ],
        });

        assert.strictEqual(await group.getOutput("org/app/prod", "url").promise(), "https://app.example.com");
        assert.strictEqual(await group.getOutput("org/app/prod", "url").isSecret, false);
        assert.strictEqual(await group.requireOutput("org/db/prod", "password").isSecret, true);
        assert.deepStrictEqual(await group.stackOutputs("org/app/prod").promise(), appOutputs);
        await assert.rejects(
            group.stackOutputs("org/web/prod").promise(),
            /Stack 'org\/web\/prod' is not a member of this stack group/,
        );
    });
});
