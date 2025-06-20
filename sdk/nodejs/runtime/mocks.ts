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

import { deserializeProperties, serializeProperties } from "./rpc";
import { getProject, getStack, setMockOptions } from "./settings";
import { getStore } from "./state";

import * as structproto from "google-protobuf/google/protobuf/struct_pb";
import * as provproto from "../proto/provider_pb";
import * as resproto from "../proto/resource_pb";

/**
 * {@link MockResourceArgs} is used to construct a new resource mock.
 */
export interface MockResourceArgs {
    /**
     * The token that indicates which resource type is being constructed. This
     * token is of the form "package:module:type".
     */
    type: string;

    /**
     * The logical name of the resource instance.
     */
    name: string;

    /**
     * The inputs for the resource.
     */
    inputs: any;

    /**
     * If provided, the identifier of the provider instance being used to manage
     * this resource.
     */
    provider?: string;

    /**
     * Specifies whether or not the resource is Custom (i.e. managed by a
     * resource provider).
     */
    custom?: boolean;

    /**
     * If provided, the physical identifier of an existing resource to read or
     * import.
     */
    id?: string;
}

/**
 * {@link MockResourceResult} is the result of a new resource mock, returning a
 * physical identifier and the output properties for the resource being
 * constructed.
 */
export type MockResourceResult = {
    id: string | undefined;
    state: Record<string, any>;
};

/**
 * {@link MockResourceArgs} is used to construct call mocks.
 */
export interface MockCallArgs {
    /**
     * The token that indicates which function is being called. This token is of
     * the form "package:module:function".
     */
    token: string;

    /**
     * The arguments provided to the function call.
     */
    inputs: any;

    /**
     * If provided, the identifier of the provider instance being used to make
     * the call.
     */
    provider?: string;
}

/**
 * {@link MockCallResult} is the result of a call mock.
 */
export type MockCallResult = Record<string, any>;

/**
 * {@link Mocks} allows implementations to replace operations normally
 * implemented by the Pulumi engine with their own implementations. This can be
 * used during testing to ensure that calls to provider functions and resource
 * constructors return predictable values.
 */
export interface Mocks {
    /**
     * Mocks provider-implemented function calls (e.g. `aws.get_availability_zones`).
     *
     * @param args MockCallArgs
     */
    call(args: MockCallArgs): MockCallResult | Promise<MockCallResult>;

    /**
     * Mocks resource construction calls. This function should return the
     * physical identifier and the output properties for the resource being
     * constructed.
     *
     * @param args MockResourceArgs
     */
    newResource(args: MockResourceArgs): MockResourceResult | Promise<MockResourceResult>;
}

export class MockMonitor {
    readonly resources = new Map<string, { urn: string; id: string | null; state: any }>();

    constructor(readonly mocks: Mocks) {}

    private newUrn(parent: string, type: string, name: string): string {
        if (parent) {
            const qualifiedType = parent.split("::")[2];
            const parentType = qualifiedType.split("$").pop();
            type = parentType + "$" + type;
        }
        return "urn:pulumi:" + [getStack(), getProject(), type, name].join("::");
    }

    public async invoke(req: any, callback: (err: any, innerResponse: any) => void) {
        try {
            const tok = req.getTok();
            const inputs = deserializeProperties(req.getArgs());

            if (tok === "pulumi:pulumi:getResource") {
                const registeredResource = this.resources.get(inputs.urn);
                if (!registeredResource) {
                    throw new Error(`unknown resource ${inputs.urn}`);
                }
                const resp = new provproto.InvokeResponse();
                resp.setReturn(structproto.Struct.fromJavaScript(registeredResource));
                callback(null, resp);
                return;
            }

            const result: MockCallResult = await this.mocks.call({
                token: tok,
                inputs: inputs,
                provider: req.getProvider(),
            });
            const response = new provproto.InvokeResponse();
            response.setReturn(structproto.Struct.fromJavaScript(await serializeProperties("", result)));
            callback(null, response);
        } catch (err) {
            callback(err, undefined);
        }
    }

    public async readResource(req: any, callback: (err: any, innterResponse: any) => void) {
        try {
            const result: MockResourceResult = await this.mocks.newResource({
                type: req.getType(),
                name: req.getName(),
                inputs: deserializeProperties(req.getProperties()),
                provider: req.getProvider(),
                custom: true,
                id: req.getId(),
            });

            const urn = this.newUrn(req.getParent(), req.getType(), req.getName());
            const serializedState = await serializeProperties("", result.state);

            this.resources.set(urn, { urn, id: result.id ?? null, state: serializedState });

            const response = new resproto.ReadResourceResponse();
            response.setUrn(urn);
            response.setProperties(structproto.Struct.fromJavaScript(serializedState));
            callback(null, response);
        } catch (err) {
            callback(err, undefined);
        }
    }

    public async registerResource(req: any, callback: (err: any, innerResponse: any) => void) {
        try {
            const result: MockResourceResult = await this.mocks.newResource({
                type: req.getType(),
                name: req.getName(),
                inputs: deserializeProperties(req.getObject()),
                provider: req.getProvider(),
                custom: req.getCustom(),
                id: req.getImportid(),
            });

            const urn = this.newUrn(req.getParent(), req.getType(), req.getName());
            const serializedState = await serializeProperties("", result.state);

            this.resources.set(urn, { urn, id: result.id ?? null, state: serializedState });

            const response = new resproto.RegisterResourceResponse();
            response.setUrn(urn);
            response.setId(result.id || "");
            response.setObject(structproto.Struct.fromJavaScript(serializedState));
            callback(null, response);
        } catch (err) {
            callback(err, undefined);
        }
    }

    public registerResourceOutputs(req: any, callback: (err: any, innerResponse: any) => void) {
        try {
            const registeredResource = this.resources.get(req.getUrn());
            if (!registeredResource) {
                throw new Error(`unknown resource ${req.getUrn()}`);
            }
            registeredResource.state = req.getOutputs();

            callback(null, {});
        } catch (err) {
            callback(err, undefined);
        }
    }

    public supportsFeature(req: any, callback: (err: any, innerResponse: any) => void) {
        const id = req.getId();

        // Support for "outputValues" is deliberately disabled for the mock monitor so
        // instances of `Output` don't show up in `MockResourceArgs` inputs.
        const hasSupport = id !== "outputValues";

        callback(null, {
            getHassupport: () => hasSupport,
        });
    }

    public registerPackage(req: any, callback: (err: any, innerResponse: any) => void) {
        // Mocks don't _really_ support packages, so we just return a fake package ref.
        const resp = new resproto.RegisterPackageResponse();
        resp.setRef("mock-uuid");
        callback(null, resp);
    }
}

/**
 * Configures the Pulumi runtime to use the given mocks for testing.
 *
 * @param mocks
 *  The mocks to use for calls to provider functions and resource construction.
 * @param project
 *  If provided, the name of the Pulumi project. Defaults to "project".
 * @param stack
 *  If provided, the name of the Pulumi stack. Defaults to "stack".
 * @param preview
 *  If provided, indicates whether or not the program is running a preview. Defaults to false.
 * @param organization
 *  If provided, the name of the Pulumi organization. Defaults to nothing.
 */
export async function setMocks(
    mocks: Mocks,
    project?: string,
    stack?: string,
    preview?: boolean,
    organization?: string,
) {
    setMockOptions(new MockMonitor(mocks), project, stack, preview, organization);

    // Mocks enable all features except outputValues.
    const store = getStore();
    store.supportsSecrets = true;
    store.supportsResourceReferences = true;
    store.supportsOutputValues = false;
    store.supportsDeletedWith = true;
    store.supportsAliasSpecs = true;
    store.supportsTransforms = false;
    store.supportsParameterization = true;
}
