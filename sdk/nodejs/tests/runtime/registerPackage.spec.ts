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
import * as runtime from "../../runtime";
import * as state from "../../runtime/state";
import * as resproto from "../../proto/resource_pb";

function setMockMonitor(result: string | Error): void {
    const store = state.getStore();
    store.settings.monitor = {
        registerPackage(_req: any, cb: (err: any, resp: any) => void) {
            if (result instanceof Error) {
                cb(result, undefined);
                return;
            }
            const resp = new resproto.RegisterPackageResponse();
            resp.setRef(result);
            cb(null, resp);
        },
    } as any;
    store.supportsParameterization = true;
}

const baseArgs: runtime.RegisterPackageArgs = {
    baseProviderName: "base",
    baseProviderVersion: "1.2.3",
    baseProviderDownloadUrl: "",
    packageName: "mypackage",
    packageVersion: "2.0.0",
    base64Parameter: "aGVsbG8=",
};

describe("runtime/registerPackage", () => {
    it("registers and caches the ref", async () => {
        await state.withLocalStorage(async () => {
            setMockMonitor("uuid-1");
            const ref = await runtime.registerPackage(baseArgs);
            assert.strictEqual(ref, "uuid-1");
            const cache = state.getPackageRefs();
            assert.strictEqual(cache.size, 1);
            assert.strictEqual(await [...cache.values()][0], "uuid-1");
        });
    });

    it("returns the same ref for the same args", async () => {
        await state.withLocalStorage(async () => {
            setMockMonitor("uuid-1");
            const a = await runtime.registerPackage(baseArgs);
            const b = await runtime.registerPackage(baseArgs);
            assert.strictEqual(a, b);
            assert.strictEqual(state.getPackageRefs().size, 1);
        });
    });

    it("uses a distinct cache entry per identifying field", async () => {
        await state.withLocalStorage(async () => {
            setMockMonitor("uuid");
            await runtime.registerPackage(baseArgs);
            await runtime.registerPackage({ ...baseArgs, packageVersion: "3.0.0" });
            await runtime.registerPackage({ ...baseArgs, base64Parameter: "T3RoZXI=" });
            await runtime.registerPackage({ ...baseArgs, baseProviderDownloadUrl: "https://example/x" });
            assert.strictEqual(state.getPackageRefs().size, 4);
        });
    });

    it("caches errors so failed registrations are not retried", async () => {
        const expected = new Error("registration failed");
        await state.withLocalStorage(async () => {
            setMockMonitor(expected);
            await assert.rejects(runtime.registerPackage(baseArgs), expected);
            await assert.rejects(runtime.registerPackage(baseArgs), expected);
            assert.strictEqual(state.getPackageRefs().size, 1);
        });
    });

    it("isolates refs between deployments", async () => {
        const refA = await state.withLocalStorage(async () => {
            setMockMonitor("uuid-A");
            return runtime.registerPackage(baseArgs);
        });
        const refB = await state.withLocalStorage(async () => {
            setMockMonitor("uuid-B");
            return runtime.registerPackage(baseArgs);
        });
        assert.strictEqual(refA, "uuid-A");
        assert.strictEqual(refB, "uuid-B");
    });

    it("throws when the engine does not support parameterization", async () => {
        await state.withLocalStorage(async () => {
            setMockMonitor("uuid");
            state.getStore().supportsParameterization = false;
            assert.throws(() => runtime.registerPackage(baseArgs), /does not support parameterization/);
        });
    });

    it("throws when there is no monitor", async () => {
        await state.withLocalStorage(async () => {
            state.getStore().supportsParameterization = true;
            assert.throws(() => runtime.registerPackage(baseArgs), /No monitor available/);
        });
    });
});
