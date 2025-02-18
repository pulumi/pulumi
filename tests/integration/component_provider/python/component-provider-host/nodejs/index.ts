// Copyright 2025, Pulumi Corporation.
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
import * as provider from "@pulumi/provider";

let comp = new provider.MyComponent("comp", {
    strInput: "hello",
    optionalIntInput: 42,
    dictInput: { a: 1, b: 2, c: 3 },
    listInput: ["a", "b", "c"],
    complexInput: {
        nestedInput: {
            strPlain: "this is nested",
        },
        strInput: "world",
    },
    assetInput: new pulumi.asset.StringAsset("Hello, World"),
    archiveInput: new pulumi.asset.AssetArchive({
        asset1: new pulumi.asset.StringAsset("im inside an archive"),
    }),
})

export const urn = comp.urn;
export const strOutput = comp.strOutput;
export const optionalIntOutput = comp.optionalIntOutput;
export const complexOutput = comp.complexOutput;
export const listOutput = comp.listOutput;
export const dictOutput = comp.dictOutput;
export const assetOutput = comp.assetOutput;
export const archiveOutput = comp.archiveOutput;
