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
import * as gstruct from "google-protobuf/google/protobuf/struct_pb";

import { ComponentResource, StateMigration } from "../../resource";
import { CallbackServer, ICallbackServer } from "../../runtime/callbacks";
import { invoke } from "../../runtime/invoke";
import * as state from "../../runtime/state";
import * as callrpc from "../../proto/callback_grpc_pb";
import { Callback, CallbackInvokeRequest } from "../../proto/callback_pb";
import * as resproto from "../../proto/resource_pb";

function invokeMigration(callback: Callback, request: resproto.StateMigrationRequest) {
    const client = new callrpc.CallbacksClient(callback.getTarget(), grpc.credentials.createInsecure());
    const invokeRequest = new CallbackInvokeRequest();
    invokeRequest.setToken(callback.getToken());
    invokeRequest.setRequest(request.serializeBinary());

    return new Promise<resproto.StateMigrationResponse>((resolve, reject) => {
        client.invoke(invokeRequest, (err, response) => {
            client.close();
            if (err) {
                reject(err);
                return;
            }
            resolve(resproto.StateMigrationResponse.deserializeBinary(response.getResponse_asU8()));
        });
    });
}

function migrationRequest(urn: string, oldState: Record<string, any>[]) {
    const request = new resproto.StateMigrationRequest();
    request.setUrn(urn);
    request.setOldState(Buffer.from(JSON.stringify(oldState), "utf8"));
    return request;
}

function recordingCallbacks(migrations: StateMigration[]): ICallbackServer {
    return {
        async registerStateMigration(migration: StateMigration) {
            migrations.push(migration);
            const callback = new Callback();
            callback.setToken(`migration-${migrations.length}`);
            callback.setTarget("127.0.0.1:1234");
            return callback;
        },
        async registerTransform() {
            throw new Error("unexpected transform registration");
        },
        registerStackTransform() {
            throw new Error("unexpected stack transform registration");
        },
        registerStackInvokeTransform() {
            throw new Error("unexpected stack invoke transform registration");
        },
        async registerStackInvokeTransformAsync() {
            throw new Error("unexpected stack invoke transform registration");
        },
        async registerResourceHook() {
            throw new Error("unexpected resource hook registration");
        },
        async registerErrorHook() {
            throw new Error("unexpected error hook registration");
        },
        shutdown: () => undefined,
        awaitStackRegistrations: async () => undefined,
    };
}

describe("state migrations", () => {
    it("round-trips migrated state, successors, and no-op results", async () => {
        const componentUrn = "urn:pulumi:stack::project::example:index:Component::component";
        const oldChildA = `${componentUrn}$example:index:Old::child-a`;
        const oldChildB = `${componentUrn}$example:index:Old::child-b`;
        const newChild = `${componentUrn}$example:index:New::child`;
        const oldState = [
            { urn: componentUrn, type: "example:index:Component" },
            { urn: oldChildA, type: "example:index:Old", custom: true, id: "id-a" },
            { urn: oldChildB, type: "example:index:Old", custom: true, id: "id-b" },
        ];

        const server = new CallbackServer({} as any);
        try {
            const migration = await server.registerStateMigration((args) => {
                assert.strictEqual(args.urn, componentUrn);
                assert.deepStrictEqual(args.oldState, oldState);

                const migratedState = args.oldState.slice(0, 2).map((resource) => ({ ...resource }));
                migratedState[1].urn = newChild;
                migratedState[1].type = "example:index:New";
                return {
                    newState: migratedState,
                    successors: {
                        [oldChildA]: newChild,
                        [oldChildB]: newChild,
                    },
                };
            });
            const response = await invokeMigration(migration, migrationRequest(componentUrn, oldState));

            const newState = JSON.parse(Buffer.from(response.getNewState_asU8()).toString("utf8"));
            assert.strictEqual(newState[1].urn, newChild);
            assert.deepStrictEqual(Object.fromEntries(response.getSuccessorsMap().entries()), {
                [oldChildA]: newChild,
                [oldChildB]: newChild,
            });

            const noop = await server.registerStateMigration(async () => undefined);
            const noopResponse = await invokeMigration(noop, migrationRequest(componentUrn, oldState));
            assert.strictEqual(noopResponse.getNewState_asU8().length, 0);
            assert.strictEqual(noopResponse.getSuccessorsMap().getLength(), 0);
        } finally {
            server.shutdown();
        }
    });

    it("rejects Pulumi runtime operations inside a migration", async () => {
        const componentUrn = "urn:pulumi:stack::project::example:index:Component::component";
        const oldState = [{ urn: componentUrn, type: "example:index:Component" }];
        const cases: [string, StateMigration][] = [
            ["resource construction", () => new ComponentResource("example:index:Nested", "nested") as any],
            ["invoke", async () => invoke("example:index:getThing", {}) as any],
        ];

        const server = new CallbackServer({} as any);
        try {
            for (const [operation, migration] of cases) {
                const callback = await server.registerStateMigration(migration);
                await assert.rejects(
                    invokeMigration(callback, migrationRequest(componentUrn, oldState)),
                    (err: grpc.ServiceError) => {
                        assert.match(err.details, new RegExp(`Pulumi runtime operation '${operation}' is not allowed`));
                        assert.ok(err.details.includes(componentUrn));
                        return true;
                    },
                );
            }
        } finally {
            server.shutdown();
        }
    });

    it("sends state migration callbacks with resource registration", async () => {
        await state.withLocalStorage(async () => {
            const migrations: StateMigration[] = [];
            let request: resproto.RegisterResourceRequest | undefined;
            const store = state.getStore();
            store.supportsStateMigrations = true;
            store.callbacks = recordingCallbacks(migrations);
            store.settings.monitor = {
                registerResource(req: resproto.RegisterResourceRequest, callback: (err: any, response: any) => void) {
                    request = req;
                    const response = new resproto.RegisterResourceResponse();
                    response.setUrn(`urn:pulumi:stack::project::${req.getType()}::${req.getName()}`);
                    response.setObject(new gstruct.Struct());
                    callback(null, response);
                },
            } as any;

            const migration: StateMigration = () => undefined;
            const component = new ComponentResource(
                "example:index:Component",
                "component",
                {},
                { stateMigrations: [migration] },
                true,
            );
            await component.urn.promise();

            assert.deepStrictEqual(migrations, [migration]);
            assert.strictEqual(request?.getStateMigrationsList().length, 1);
            assert.strictEqual(request?.getStateMigrationsList()[0].getToken(), "migration-1");
        });
    });

    it("rejects a monitor without state migration support", async () => {
        await state.withLocalStorage(async () => {
            const migrations: StateMigration[] = [];
            let registered = false;
            const store = state.getStore();
            store.callbacks = recordingCallbacks(migrations);
            store.settings.monitor = {
                registerResource(_req: any, callback: (err: any, response: any) => void) {
                    registered = true;
                    callback(null, new resproto.RegisterResourceResponse());
                },
            } as any;

            assert.throws(
                () =>
                    new ComponentResource(
                        "example:index:Component",
                        "component",
                        {},
                        { stateMigrations: [() => undefined] },
                        true,
                    ),
                /does not support state migrations/,
            );
            assert.deepStrictEqual(migrations, []);
            assert.strictEqual(registered, false);
        });
    });
});
