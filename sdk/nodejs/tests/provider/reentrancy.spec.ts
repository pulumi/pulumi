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

import * as pulumi from "../..";
import { ConstructResult, Provider } from "../../provider";
import { Server } from "../../provider/server";

import * as gstruct from "google-protobuf/google/protobuf/struct_pb";
import * as provproto from "../../proto/provider_pb";

/** A deferred promise handle. */
function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void } {
    let resolve!: (v: T) => void;
    const promise = new Promise<T>((res) => {
        resolve = res;
    });
    return { promise, resolve };
}

function invokeConstruct(server: Server, project: string, type: string, name: string): Promise<any> {
    const req = new provproto.ConstructRequest();
    req.setProject(project);
    req.setStack("stack");
    req.setParallel(1);
    // An empty monitor endpoint keeps the runtime offline: awaitFeatureSupport
    // no-ops and the provider's construct never talks to a real monitor.
    req.setMonitorendpoint("");
    req.setDryrun(false);
    req.setOrganization("");
    req.setType(type);
    req.setName(name);
    req.setInputs(new gstruct.Struct());
    return new Promise((resolve, reject) => {
        server.construct({ request: req }, (err: any, resp: provproto.ConstructResponse) => {
            if (err) {
                reject(err instanceof Error ? err : new Error(JSON.stringify(err)));
            } else {
                resolve(resp.getState()!.toJavaScript());
            }
        });
    });
}

function invokeConstructBase(server: Server, project: string, type: string, name: string, urn: string): Promise<any> {
    const req = new provproto.ConstructBaseRequest();
    req.setProject(project);
    req.setStack("stack");
    req.setParallel(1);
    req.setMonitorEndpoint("");
    req.setDryRun(false);
    req.setOrganization("");
    req.setType(type);
    req.setName(name);
    req.setUrn(urn);
    req.setInputs(new gstruct.Struct());
    return new Promise((resolve, reject) => {
        server.constructBase({ request: req }, (err: any, resp: provproto.ConstructBaseResponse) => {
            if (err) {
                reject(err instanceof Error ? err : new Error(JSON.stringify(err)));
            } else {
                resolve(resp.getState()!.toJavaScript());
            }
        });
    });
}

describe("provider server reentrancy", () => {
    it("isolates per-request state across overlapping construct and constructBase", async () => {
        const gate = deferred<void>();
        let entered = 0;
        const bothEntered = deferred<void>();

        // Each request records the project it saw before and after blocking on the
        // shared gate; if per-request async-local state were shared, the second
        // request's configureRuntime would clobber the first's project.
        const provider: Provider = {
            version: "0.0.0",
            async construct(name, type, _inputs, _options): Promise<ConstructResult> {
                const project = pulumi.runtime.getProject();
                if (++entered === 2) {
                    bothEntered.resolve();
                }
                await gate.promise;
                assert.strictEqual(pulumi.runtime.getProject(), project, "per-request project leaked across requests");
                return {
                    urn: pulumi.output(`urn:pulumi:stack::${project}::${type}::${name}`),
                    state: { seenProject: project },
                };
            },
        };

        const server = new Server("127.0.0.1:1", provider, new Set<Error>());

        const a = invokeConstruct(server, "projA", "pkg:index:A", "a");
        const b = invokeConstructBase(server, "projB", "pkg:index:B", "b", "urn:pulumi:stack::projB::pkg:index:B::b");

        // Both requests must be parked inside construct before we release the gate;
        // that they both got here at all proves the second was not serialized behind
        // the first (no deadlock).
        await bothEntered.promise;
        gate.resolve();

        const [aState, bState] = await Promise.all([a, b]);
        assert.strictEqual(aState.seenProject, "projA");
        assert.strictEqual(bState.seenProject, "projB");
    });
});
