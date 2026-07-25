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

import * as pulumi from "../..";
import * as resource from "../../resource";
import * as runtime from "../../runtime";
import * as state from "../../runtime/state";

import * as emptyproto from "google-protobuf/google/protobuf/empty_pb";
import * as gstruct from "google-protobuf/google/protobuf/struct_pb";
import * as resproto from "../../proto/resource_pb";

interface MonitorCalls {
    registerResource: number;
    registerResourceOutputs: number;
    invoke: number;
    constructBaseResource: number;
    getDeploymentInfo: number;
}

type ConstructBaseHandler = (
    req: resproto.ConstructBaseResourceRequest,
) => resproto.ConstructBaseResourceResponse | Error;

/** Builds an Error shaped like a gRPC ServiceError with the given status code. */
function grpcError(code: number, message: string): Error {
    return Object.assign(new Error(message), { code });
}

interface InstalledMonitor {
    calls: MonitorCalls;
    registerParents: string[];
    lastConstructBase: () => resproto.ConstructBaseResourceRequest | undefined;
}

/**
 * Installs a mock resource monitor on the current store. `getDeploymentInfo`
 * advertises base-construction support per `supportsConstructBase`, and any
 * `registerResource`/`getResource` traffic is counted so tests can assert the
 * attach path never registers or fetches.
 */
function installMonitor(cfg: {
    supportsConstructBase?: boolean;
    onConstructBase?: ConstructBaseHandler;
}): InstalledMonitor {
    const calls: MonitorCalls = {
        registerResource: 0,
        registerResourceOutputs: 0,
        invoke: 0,
        constructBaseResource: 0,
        getDeploymentInfo: 0,
    };
    const registerParents: string[] = [];
    let lastConstructBase: resproto.ConstructBaseResourceRequest | undefined;

    const monitor = {
        getDeploymentInfo(_req: any, cb: (err: any, resp: any) => void) {
            calls.getDeploymentInfo++;
            const info = new resproto.DeploymentInfo();
            if (cfg.supportsConstructBase) {
                info.addSupportedfeatures(resproto.ResourceMonitorFeature.RESOURCE_MONITOR_FEATURE_CONSTRUCT_BASE);
            }
            cb(null, info);
        },
        constructBaseResource(req: resproto.ConstructBaseResourceRequest, cb: (err: any, resp: any) => void) {
            calls.constructBaseResource++;
            lastConstructBase = req;
            const result = cfg.onConstructBase!(req);
            if (result instanceof resproto.ConstructBaseResourceResponse) {
                cb(null, result);
            } else {
                cb(result, undefined);
            }
        },
        registerResource(req: any, cb: (err: any, resp: any) => void) {
            calls.registerResource++;
            registerParents.push(req.getParent());
            const resp = new resproto.RegisterResourceResponse();
            resp.setUrn(`urn:pulumi:stack::project::${req.getType()}::${req.getName()}`);
            resp.setObject(req.getObject());
            resp.setResult(resproto.Result.SUCCESS);
            cb(null, resp);
        },
        registerResourceOutputs(_req: any, cb: (err: any, resp: any) => void) {
            calls.registerResourceOutputs++;
            cb(null, new emptyproto.Empty());
        },
        invoke(_req: any, cb: (err: any, resp: any) => void) {
            // The attach path must never fall back to the getResource invoke.
            calls.invoke++;
            cb(new Error("unexpected getResource/invoke call"), undefined);
        },
    };

    state.getStore().settings.monitor = monitor as any;
    return { calls, registerParents, lastConstructBase: () => lastConstructBase };
}

class ChildResource extends pulumi.CustomResource {
    constructor(name: string, opts?: pulumi.CustomResourceOptions) {
        super("pkgB:index:Child", name, {}, opts);
    }
}

/** Builds a `ConstructBaseResourceResponse` carrying the given state. */
function stateResponse(s: Record<string, any>): resproto.ConstructBaseResourceResponse {
    const resp = new resproto.ConstructBaseResourceResponse();
    resp.setState(gstruct.Struct.fromJavaScript(s));
    return resp;
}

/** Constructs a component in attach mode, adopting `urn` without any RPC. */
function attachedComponent(type: string, name: string, urn: string): pulumi.ComponentResource {
    const opts: pulumi.ComponentResourceOptions = {};
    resource.setAttachBaseResource(opts, { urn });
    return new pulumi.ComponentResource(type, name, {}, opts);
}

const derivedUrn = "urn:pulumi:stack::project::pkgB:index:Derived::res";

describe("runtime/attach mode", () => {
    it("adopts the URN directly without registering or reading", async () => {
        await state.withLocalStorage(async () => {
            const { calls, registerParents } = installMonitor({});

            const attachUrn = "urn:pulumi:stack::project::pkgB:index:Derived::comp";
            const res = attachedComponent("pkgB:index:Derived", "comp", attachUrn);

            // The URN resolves to the adopted value with no registration/fetch.
            assert.strictEqual(await res.urn.promise(), attachUrn);
            // Let initialize() and the (suppressed) registerOutputs run.
            await (res as any).__data;

            assert.strictEqual(calls.registerResource, 0);
            assert.strictEqual(calls.invoke, 0);
            // registerOutputs is a no-op in attach mode (the resource already exists).
            assert.strictEqual(calls.registerResourceOutputs, 0);

            // A child created with parent=res sends the adopted URN as its parent.
            const child = new ChildResource("child", { parent: res });
            await child.urn.promise();
            assert.strictEqual(calls.registerResource, 1);
            assert.strictEqual(registerParents[0], attachUrn);
        });
    });

    it("an explicit registerOutputs call is still suppressed", async () => {
        await state.withLocalStorage(async () => {
            const { calls } = installMonitor({});
            const res = attachedComponent("pkgB:index:Derived", "comp", derivedUrn);
            await (res as any).__data;

            // Reaching in to call the protected registerOutputs must remain a no-op.
            (res as any).registerOutputs({ foo: "bar" });
            await new Promise((r) => setTimeout(r, 0));
            assert.strictEqual(calls.registerResourceOutputs, 0);
        });
    });
});

describe("runtime/constructBaseResource", () => {
    it("serializes inputs with output values and resolves the base outputs onto the instance", async () => {
        await state.withLocalStorage(async () => {
            const monitor = installMonitor({
                supportsConstructBase: true,
                onConstructBase: () => stateResponse({ foo: "bar", num: 42, extra: "ignored" }),
            });
            const store = state.getStore();
            store.supportsOutputValues = true;

            const res = attachedComponent("pkgB:index:Derived", "res", derivedUrn);
            // A pre-existing own property standing in for a derived-owned output.
            (res as any).preset = "original";

            await runtime.constructBaseResource(
                res,
                "pkgC:index:Mid",
                { name: pulumi.output("hello") },
                { version: "1.2.3" },
                ["foo", "num", "preset"],
            );

            // Exactly the seeded keys resolve; the derived-owned property is untouched
            // and unlisted keys returned by the engine are skipped.
            assert.strictEqual(await (res as any).foo.promise(), "bar");
            assert.strictEqual(await (res as any).num.promise(), 42);
            assert.strictEqual((res as any).preset, "original");
            assert.strictEqual((res as any).extra, undefined);

            assert.strictEqual(monitor.calls.constructBaseResource, 1);
            assert.strictEqual(monitor.calls.registerResource, 0);
            assert.strictEqual(monitor.calls.invoke, 0);

            const req = monitor.lastConstructBase()!;
            assert.strictEqual(req.getUrn(), derivedUrn);
            assert.strictEqual(req.getBaseType(), "pkgC:index:Mid");
            assert.strictEqual(req.getVersion(), "1.2.3");

            // The Output input is carried on the wire as a rich output value.
            const inputsJs: any = req.getInputs()!.toJavaScript();
            assert.strictEqual(inputsJs.name[runtime.specialSigKey], runtime.specialOutputValueSig);
            assert.strictEqual(inputsJs.name.value, "hello");
        });
    });

    it("passes a package reference through in preference to version fields", async () => {
        await state.withLocalStorage(async () => {
            const monitor = installMonitor({
                supportsConstructBase: true,
                onConstructBase: () => stateResponse({}),
            });

            const res = attachedComponent("pkgB:index:Derived", "res", derivedUrn);
            await runtime.constructBaseResource(
                res,
                "pkgC:index:Mid",
                {},
                { version: "1.2.3", packageRef: Promise.resolve("ref-123") },
                [],
            );

            const req = monitor.lastConstructBase()!;
            assert.strictEqual(req.getPackageRef(), "ref-123");
            assert.strictEqual(req.getVersion(), "");
        });
    });

    it("fails fast with an upgrade error when the engine lacks base-construct support", async () => {
        await state.withLocalStorage(async () => {
            const monitor = installMonitor({ supportsConstructBase: false });
            const res = attachedComponent("pkgB:index:Derived", "res", derivedUrn);

            await assert.rejects(
                runtime.constructBaseResource(res, "pkgC:index:Mid", {}, {}, ["foo"]),
                /requires a newer version of the Pulumi CLI/,
            );
            // The gate must fire before any base-construct RPC is issued.
            assert.strictEqual(monitor.calls.constructBaseResource, 0);
        });
    });

    it("maps an Unimplemented base-construct RPC to the same upgrade error", async () => {
        await state.withLocalStorage(async () => {
            installMonitor({
                supportsConstructBase: true,
                onConstructBase: () => grpcError(grpc.status.UNIMPLEMENTED, "unimplemented"),
            });
            const res = attachedComponent("pkgB:index:Derived", "res", derivedUrn);

            await assert.rejects(
                runtime.constructBaseResource(res, "pkgC:index:Mid", {}, {}, ["foo"]),
                /requires a newer version of the Pulumi CLI/,
            );
        });
    });

    it("propagates a base-construct RPC failure to the caller and its seeded outputs", async () => {
        await state.withLocalStorage(async () => {
            installMonitor({
                supportsConstructBase: true,
                onConstructBase: () => grpcError(grpc.status.INTERNAL, "boom"),
            });
            const res = attachedComponent("pkgB:index:Derived", "res", derivedUrn);

            const p = runtime.constructBaseResource(res, "pkgC:index:Mid", {}, {}, ["foo"]);
            await assert.rejects(p, /boom/);
            // The seeded output must also reject (a dependent awaiting a base output
            // fails rather than hanging).
            await assert.rejects((res as any).foo.promise(), /boom/);
        });
    });

    it("seeds the base output cells synchronously, before the RPC resolves", async () => {
        await state.withLocalStorage(async () => {
            installMonitor({
                supportsConstructBase: true,
                onConstructBase: () => stateResponse({ foo: "bar" }),
            });
            const res = attachedComponent("pkgB:index:Derived", "res", derivedUrn);

            // The call returns after seeding, before the async RPC/resolution: a constructor-context caller can read
            // the base's Output field on the very next line without awaiting the returned promise.
            const p = runtime.constructBaseResource(res, "pkgC:index:Mid", {}, {}, ["foo"]);
            assert.notStrictEqual((res as any).foo, undefined, "base output cell must exist synchronously");
            assert.ok(pulumi.Output.isInstance((res as any).foo), "base output must be an Output cell");

            await p;
            assert.strictEqual(await (res as any).foo.promise(), "bar");
        });
    });
});
