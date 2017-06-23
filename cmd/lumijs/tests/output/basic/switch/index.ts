// Copyright 2016-2017, Pulumi Corporation
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

// This tests the code-generated expansion of a switch statement.

function sw(v: string): string {
    let result: string = "";
    switch (v) {
        case "a":
            result += "a";
            break;
        case "b":
            result += "b";
            // intentional fallthrough.
        default:
            result += "d";
            break;
    }
    return result;
}

let a = sw("a");
if (a !== "a") {
    throw "Expected 'a'; got '" + a + "'";
}

let b = sw("b");
if (b !== "bd") {
    throw "Expected 'bd'; got '" + b + "'";
}

let d = sw("d");
if (d !== "d") {
    throw "Expected 'd'; got '" + d + "'";
}

