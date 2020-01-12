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

const provproto = require("../proto/provider_pb.js");
const resproto = require("../proto/resource_pb.js");
const structproto = require("google-protobuf/google/protobuf/struct_pb.js");

export interface Mocks {
    call(token: string, args: any, provider?: string): any;
    newResource(type: string, name: string, inputs: any, provider?: string, id?: string): { id: string, state: any };
}

export class MockMonitor {
    mocks: Mocks;

    constructor(mocks: Mocks) {
        this.mocks = mocks;
    }

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
            const result = this.mocks.call(req.getToken(), deserializeProperties(req.getArgs()), req.getProvider());
            const response = new provproto.InvokeResponse();
            response.setReturn(structproto.Struct.fromJavaScript(await serializeProperties("", result)));
            callback(null, response);
        } catch (err) {
            callback(err, undefined);
        }
    }

    public async readResource(req: any, callback: (err: any, innterResponse: any) => void) {
        try {
            const result = this.mocks.newResource(
                req.getType(),
                req.getName(),
                deserializeProperties(req.getProperties()),
                req.getProvider(),
                req.getId());
            const response = new resproto.ReadResourceResponse();
            response.setUrn(this.newUrn(req.getParent(), req.getType(), req.getName()));
            response.setProperties(structproto.Struct.fromJavaScript(await serializeProperties("", result.state)));
            callback(null, response);
        } catch (err) {
            callback(err, undefined);
        }
    }

    public async registerResource(req: any, callback: (err: any, innerResponse: any) => void) {
        try {
            const result = this.mocks.newResource(
                req.getType(),
                req.getName(),
                deserializeProperties(req.getObject()),
                req.getProvider(),
                req.getImportid());
            const response = new resproto.RegisterResourceResponse();
            response.setUrn(this.newUrn(req.getParent(), req.getType(), req.getName()));
            response.setId(result.id);
            response.setObject(structproto.Struct.fromJavaScript(await serializeProperties("", result.state)));
            callback(null, response);
        } catch (err) {
            callback(err, undefined);
        }
    }

    public registerResourceOutputs(req: any, callback: (err: any, innerResponse: any) => void) {
        callback(null, {});
    }
}

export function setMocks(mocks: Mocks, project?: string, stack?: string, preview?: boolean) {
    setMockOptions(new MockMonitor(mocks), project || "project", stack || "stack", preview);
}
