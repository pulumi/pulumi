// Copyright 2024-2024, Pulumi Corporation.
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

export const description = "Cloud table function";

const table1: any = { primaryKey: 1, insert: () => { }, scan: () => { } };

async function testScanReturnsAllValues() {
    await table1.insert({ [table1.primaryKey.get()]: "val1", value1: 1, value2: "1" });
    await table1.insert({ [table1.primaryKey.get()]: "val2", value1: 2, value2: "2" });

    const values = null;
    // @ts-ignore
    const value1 = values.find(v => v[table1.primaryKey.get()] === "val1");
    // @ts-ignore
    const value2 = values.find(v => v[table1.primaryKey.get()] === "val2");
}

export const func = testScanReturnsAllValues;