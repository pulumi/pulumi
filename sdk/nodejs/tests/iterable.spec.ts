// Copyright 2016-2018, Pulumi Corporation.
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
import * as iterable from "../iterable";
import { Output } from "../output";
import { asyncTest } from "./util";

describe("iterable", () => {
    it("toMap does its job", asyncTest(async () => {
        interface Instance {
            id: Output<string>;
            privateIp: Output<string>;
        }

        const instances: Instance[] = [
            { id: Output.create("i-1234"), privateIp: Output.create("192.168.1.2") },
            { id: Output.create("i-5678"), privateIp: Output.create("192.168.1.5") },
        ];

        const result = iterable.toObject(instances, i => [i.id, i.privateIp]);
        const isKnown = await result.isKnown;
        assert.equal(isKnown, true);
        const value = await result.promise();
        assert.deepEqual(value, { "i-1234": "192.168.1.2", "i-5678": "192.168.1.5" });
    }));
    it("groupBy does its job", asyncTest(async () => {
        interface Instance {
            id: Output<string>;
            availabilityZone: Output<string>;
        }

        const instances: Instance[] = [
            { id: Output.create("i-1234"), availabilityZone: Output.create("us-east-1a") },
            { id: Output.create("i-1538"), availabilityZone: Output.create("us-west-2c") },
            { id: Output.create("i-5678"), availabilityZone: Output.create("us-east-1a") },
        ];

        const result = iterable.groupBy(instances, i => [i.availabilityZone, i.id]);
        const isKnown = await result.isKnown;
        assert.equal(isKnown, true);
        const value = await result.promise();
        assert.deepEqual(value, { "us-east-1a": ["i-1234", "i-5678"], "us-west-2c": ["i-1538"] });
    }));
});
