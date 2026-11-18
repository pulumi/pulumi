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
import { Config } from "../../config";
import { isScanUnknown, policyScanState, scanInvokeResult, scanPartial, scanUnknown } from "../../runtime/policyScan";
import { serializeProperty, unknownValue } from "../../runtime/rpc";

describe("policy scan", () => {
    // An empty value turns scan mode off.
    let saved: string | undefined;
    beforeEach(() => {
        saved = process.env.PULUMI_POLICY_SCAN;
        process.env.PULUMI_POLICY_SCAN = "1";
        (globalThis as any).__pulumiPolicyScan = undefined;
    });
    afterEach(() => {
        process.env.PULUMI_POLICY_SCAN = saved ?? "";
        (globalThis as any).__pulumiPolicyScan = undefined;
    });

    it("gives an unknown that tolerates any use", () => {
        const u = scanUnknown();
        assert.ok(isScanUnknown(u));
        assert.ok(isScanUnknown(u.a.b.c));
        assert.ok(isScanUnknown(u.method(1, 2)));
        assert.ok(isScanUnknown(new u.Thing()));
        assert.deepStrictEqual([...u], []);
        assert.strictEqual(u.length, 0);
        assert.strictEqual(`${u}`, "[unknown]");
        // Not a thenable, so awaiting it doesn't hang.
        assert.strictEqual(u.then, undefined);
        assert.ok(!isScanUnknown("x"));
        assert.ok(!isScanUnknown(undefined));
    });

    it("serializes an unknown as the unknown sentinel", async () => {
        assert.strictEqual(await serializeProperty("p", scanUnknown(), new Set()), unknownValue);
        const obj = await serializeProperty("p", { a: "known", b: scanUnknown().id }, new Set());
        assert.deepStrictEqual(obj, { a: "known", b: unknownValue });
    });

    it("returns a fixture for an invoke, by token or function name", () => {
        policyScanState().invokes = { getAccount: { id: "123", nested: { region: "us-east-1" } } };

        const { result, isKnown } = scanInvokeResult("aws:index/getAccount:getAccount");
        assert.ok(isKnown);
        assert.strictEqual(result.id, "123");
        assert.strictEqual(result.nested.region, "us-east-1");
        assert.ok(isScanUnknown(result.missing), "fields the fixture leaves out are unknown");
        assert.ok(isScanUnknown(result.nested.missing));

        const other = scanInvokeResult("aws:ec2/getVpc:getVpc");
        assert.ok(!other.isKnown);
        assert.ok(isScanUnknown(other.result));

        const state = policyScanState();
        assert.deepStrictEqual([...state.usedFixtures], ["invoke getAccount"]);
        assert.deepStrictEqual([...state.unknownInvokes], ["aws:ec2/getVpc:getVpc"]);
    });

    it("leaves values a fixture can't wrap alone", () => {
        assert.strictEqual(scanPartial("s"), "s");
        assert.deepStrictEqual(scanPartial([1, 2]), [1, 2]);
        assert.strictEqual(scanPartial(null), null);
    });

    it("gives required config nobody set a placeholder", () => {
        const config = new Config("scan-spec");
        assert.strictEqual(config.require("name"), "[missing config scan-spec:name]");
        assert.strictEqual(config.requireNumber("count"), 0);
        assert.strictEqual(config.requireBoolean("enabled"), false);
        assert.deepStrictEqual(config.requireObject("settings"), {});
        assert.deepStrictEqual([...policyScanState().missingConfig].sort(), [
            "scan-spec:count",
            "scan-spec:enabled",
            "scan-spec:name",
            "scan-spec:settings",
        ]);
    });

    it("still throws for missing config outside a scan", () => {
        process.env.PULUMI_POLICY_SCAN = "";
        assert.throws(() => new Config("scan-spec").require("name"), /Missing required configuration variable/);
    });
});
