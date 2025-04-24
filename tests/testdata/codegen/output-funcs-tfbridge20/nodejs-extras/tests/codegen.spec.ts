// Copyright 2016-2021, Pulumi Corporation.
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

import "mocha";
import * as assert from "assert";

import * as pulumi from "@pulumi/pulumi";

import { listStorageAccountKeysOutput, ListStorageAccountKeysResult } from "../listStorageAccountKeys";
import * as amiIds from "../getAmiIds";
import { GetAmiIdsFilterArgs } from "../types/input";

pulumi.runtime.setMocks({
    newResource: function(_: pulumi.runtime.MockResourceArgs): {id: string, state: any} {
        throw new Error("newResource not implemented");
    },
    call: function(args: pulumi.runtime.MockCallArgs) {
        if (args.token == "mypkg::listStorageAccountKeys") {
            return {
                "keys": [{
                    "creationTime": "my-creation-time",
                    "keyName": "my-key-name",
                    "permissions": "my-permissions",
                    "value": JSON.stringify(args.inputs),
                }]
            };
        }

        if (args.token == "mypkg::getAmiIds") {
            return {
                sortAscending: args.inputs.sortAscending,
                nameRegex: args.inputs.nameRegex,
                owners: args.inputs.owners,
                id: JSON.stringify({filters: args.inputs.filters})
            }
        }

        throw new Error("call not implemented for " + args.token);
    },
});

function checkTable(done: any, transform: (res: any) => any, table: {given: pulumi.Output<any>, expect: any}[]) {
    checkOutput(done, pulumi.all(table.map(x => x.given)), res => {
        res.map((actual, i) => {
            assert.deepStrictEqual(transform(actual), table[i].expect);
        });
    });
}

describe("output-funcs", () => {

    it("getAmiIdsOutput", (done) => {

        function filter(n: number): GetAmiIdsFilterArgs {
            return {
                name: pulumi.output(`filter-${n}-name`),
                values: [
                    pulumi.output(`filter-${n}-value-1`),
                    pulumi.output(`filter-${n}-value-2`)
                ],
            }
        }

        const output = amiIds.getAmiIdsOutput({
            owners: [pulumi.output("owner-1"),
                     pulumi.output("owner-2")],
            nameRegex: pulumi.output("[a-z]"),
            sortAscending: pulumi.output(true),
            filters: [filter(1), filter(2)],
        });

        checkOutput(done, output, (res: amiIds.GetAmiIdsResult) => {
            assert.equal(res.sortAscending, true);
            assert.equal(res.nameRegex, "[a-z]");
            assert.deepStrictEqual(res.owners, ["owner-1", "owner-2"]);

            assert.deepStrictEqual(JSON.parse(res.id), {
                filters: [
                    {
                        name: 'filter-1-name',
                        values: [
                            'filter-1-value-1',
                            'filter-1-value-2'
                        ]
                    },
                    {
                        name: 'filter-2-name',
                        values: [
                            'filter-2-value-1',
                            'filter-2-value-2'
                        ]
                    },
                ]
            });
        });
    });

    it("listStorageAccountKeysOutput", (done) => {
        const output = listStorageAccountKeysOutput({
            accountName: pulumi.output("my-account-name"),
            resourceGroupName: pulumi.output("my-resource-group-name"),
        });
        checkOutput(done, output, (res: ListStorageAccountKeysResult) => {
            assert.equal(res.keys.length, 1);
            const k = res.keys[0];
            assert.equal(k.creationTime, "my-creation-time");
            assert.equal(k.keyName, "my-key-name");
            assert.equal(k.permissions, "my-permissions");
            assert.deepStrictEqual(JSON.parse(k.value), {
                "accountName": "my-account-name",
                "resourceGroupName": "my-resource-group-name"
            });
        });
    });

    it("listStorageAccountKeysOutput with optional arg set", (done) => {
        const output = listStorageAccountKeysOutput({
            accountName: pulumi.output("my-account-name"),
            resourceGroupName: pulumi.output("my-resource-group-name"),
            expand: pulumi.output("my-expand"),
        });
        checkOutput(done, output, (res: ListStorageAccountKeysResult) => {
            assert.equal(res.keys.length, 1);
            const k = res.keys[0];
            assert.equal(k.creationTime, "my-creation-time");
            assert.equal(k.keyName, "my-key-name");
            assert.equal(k.permissions, "my-permissions");
            assert.deepStrictEqual(JSON.parse(k.value), {
                "accountName": "my-account-name",
                "resourceGroupName": "my-resource-group-name",
                "expand": "my-expand"
            });
        });
    });

 });


function checkOutput<T>(done: any, output: pulumi.Output<T>, check: (value: T) => void) {
    output.apply(value => {
        try {
            check(value);
            done();
        } catch (error) {
            done(error);
        }
    });
}
