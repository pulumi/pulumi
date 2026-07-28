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
import * as grpc from "@grpc/grpc-js";
import * as resproto from "../../proto/resource_pb";
import { awaitFeatureSupport } from "../../runtime/settings";
import * as state from "../../runtime/state";

const Feature = resproto.ResourceMonitorFeature;

function storeFeatureFlags() {
    const store = state.getStore();
    return {
        supportsSecrets: store.supportsSecrets,
        supportsResourceReferences: store.supportsResourceReferences,
        supportsOutputValues: store.supportsOutputValues,
        supportsDeletedWith: store.supportsDeletedWith,
        supportsReplaceWith: store.supportsReplaceWith,
        supportsAliasSpecs: store.supportsAliasSpecs,
        supportsTransforms: store.supportsTransforms,
        supportsInvokeTransforms: store.supportsInvokeTransforms,
        supportsParameterization: store.supportsParameterization,
        supportsResourceHooks: store.supportsResourceHooks,
        supportsErrorHooks: store.supportsErrorHooks,
    };
}

describe("runtime/featureSupport", () => {
    it("loads features from GetDeploymentInfo without probing SupportsFeature", async () => {
        await state.withLocalStorage(async () => {
            let supportsFeatureCalls = 0;
            state.getStore().settings.monitor = {
                getDeploymentInfo(_req: any, cb: (err: any, resp: any) => void) {
                    const resp = new resproto.DeploymentInfo();
                    resp.setSupportedfeaturesList([
                        Feature.RESOURCE_MONITOR_FEATURE_SECRETS,
                        Feature.RESOURCE_MONITOR_FEATURE_RESOURCE_REFERENCES,
                        Feature.RESOURCE_MONITOR_FEATURE_ALIAS_SPECS,
                        Feature.RESOURCE_MONITOR_FEATURE_TRANSFORMS,
                        Feature.RESOURCE_MONITOR_FEATURE_RESOURCE_HOOKS,
                        // Features with no legacy string ID never map to a store flag.
                        Feature.RESOURCE_MONITOR_FEATURE_BYTE_STRING,
                    ]);
                    cb(null, resp);
                },
                supportsFeature(_req: any, cb: (err: any, resp: any) => void) {
                    supportsFeatureCalls++;
                    cb(null, { getHassupport: () => true });
                },
            } as any;

            await awaitFeatureSupport();

            assert.deepStrictEqual(storeFeatureFlags(), {
                supportsSecrets: true,
                supportsResourceReferences: true,
                supportsOutputValues: false,
                supportsDeletedWith: false,
                supportsReplaceWith: false,
                supportsAliasSpecs: true,
                supportsTransforms: true,
                supportsInvokeTransforms: false,
                supportsParameterization: false,
                supportsResourceHooks: true,
                supportsErrorHooks: false,
            });
            assert.strictEqual(supportsFeatureCalls, 0);
        });
    });

    it("falls back to SupportsFeature when GetDeploymentInfo is unimplemented", async () => {
        await state.withLocalStorage(async () => {
            const supported = new Set(["secrets", "outputValues", "invokeTransforms"]);
            const probed: string[] = [];
            state.getStore().settings.monitor = {
                getDeploymentInfo(_req: any, cb: (err: any, resp: any) => void) {
                    cb({ code: grpc.status.UNIMPLEMENTED }, undefined);
                },
                supportsFeature(req: any, cb: (err: any, resp: any) => void) {
                    probed.push(req.getId());
                    cb(null, { getHassupport: () => supported.has(req.getId()) });
                },
            } as any;

            await awaitFeatureSupport();

            assert.deepStrictEqual(storeFeatureFlags(), {
                supportsSecrets: true,
                supportsResourceReferences: false,
                supportsOutputValues: true,
                supportsDeletedWith: false,
                supportsReplaceWith: false,
                supportsAliasSpecs: false,
                supportsTransforms: false,
                supportsInvokeTransforms: true,
                supportsParameterization: false,
                supportsResourceHooks: false,
                supportsErrorHooks: false,
            });
            assert.deepStrictEqual(probed.sort(), [
                "aliasSpecs",
                "deletedWith",
                "errorHooks",
                "invokeTransforms",
                "outputValues",
                "parameterization",
                "replaceWith",
                "resourceHooks",
                "resourceReferences",
                "secrets",
                "transforms",
            ]);
        });
    });
});
