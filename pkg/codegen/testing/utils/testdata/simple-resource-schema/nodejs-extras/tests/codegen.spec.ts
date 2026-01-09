// Copyright 2016-2021, Pulumi Corporation.
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

import * as pulumi from "@pulumi/pulumi";
import * as assert from "assert";
import "mocha";
import * as sut from "..";

pulumi.runtime.setMocks({
    newResource: function(_: pulumi.runtime.MockResourceArgs): {id: string, state: any} {
        return {id: "", state: {}};
    },
    call: function(args: pulumi.runtime.MockCallArgs) {
        throw new Error("call not implemented");
    },
});


describe("simple-resource-schema", () => {
    const p = new sut.Provider("my-p");
    const r = new sut.Resource("my-r", {});
    const c = new sut.FooResource("my-c", {})

    assert.deepStrictEqual(sut.Provider.isInstance(p), true);
    assert.deepStrictEqual(sut.Provider.isInstance(r), false);
    assert.deepStrictEqual(sut.Provider.isInstance(c), false);

    assert.deepStrictEqual(sut.Resource.isInstance(p), false);
    assert.deepStrictEqual(sut.Resource.isInstance(r), true);
    assert.deepStrictEqual(sut.Resource.isInstance(c), false);

    assert.deepStrictEqual(sut.FooResource.isInstance(p), false);
    assert.deepStrictEqual(sut.FooResource.isInstance(r), false);
    assert.deepStrictEqual(sut.FooResource.isInstance(c), true);
 });
