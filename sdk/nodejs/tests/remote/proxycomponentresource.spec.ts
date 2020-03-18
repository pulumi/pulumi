// Copyright 2016-2020, Pulumi Corporation.
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


// import * as assert from "assert";
// import  { ComponentResourceOptions, Output } from "../../";
// import { ProxyComponentResource } from "../../remote";

import * as assert from "assert";
import { getRemoteServer, RemoteServer } from "../../remote/remoteServer";
import * as runtime from "../../runtime";

describe("remote invocation", () => {
    before(() => {
        runtime._setTestModeEnabled(true);
        runtime._setProject("myproject");
        runtime._setStack("mystack");
    });

    after(() => {
        runtime._setTestModeEnabled(false);
        runtime._setProject(undefined);
        runtime._setStack(undefined);
    });

    it("can construct a simple component", async () => {
        const server = getRemoteServer();
        const path = require.resolve("./component");
        console.log(path);
        const res = await server.construct(require.resolve("./component"), "MyComponent", "res", { input: "hello", output: undefined }, {});
        assert.strictEqual("hello", res.output);
    });
});
