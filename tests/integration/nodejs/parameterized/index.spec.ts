import * as pulumi from "@pulumi/pulumi";
import * as pkg from "@pulumi/pkg";
import { describe, it } from "mocha";
import * as assert from "assert";
class Mocks implements pulumi.runtime.Mocks {
    call(args: pulumi.runtime.MockCallArgs): Record<string, any> {
        throw new Error(`unknown function ${args.token}`);
    }

    newResource(args: pulumi.runtime.MockResourceArgs): { id: string | undefined; state: Record<string, any> } {
        return {
            id: args.name + "_id",
            state: args.inputs,
        };
    }
}

pulumi.runtime.setMocks(new Mocks(), "project", "stack", false);

describe("Parameterized", function () {
    it("should create a Random resource", async function () {
        const random = new pkg.Random("random", { length: 8 });

        const result = await new Promise((resolve, reject) => {
            random.id.apply(id => {
                if (id) {
                    resolve(id);
                } else {
                    reject(new Error("Resource ID is undefined"));
                }
            });
        });

        assert.equal("random_id", result);
    });
});