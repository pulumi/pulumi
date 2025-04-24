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

// For this case we want a function whose code contains a shadowed
// reserved identifier. The simplest way to achieve this with the least amount
// of variability across TypeScript versions is to use vanilla JavaScript with
// hand-rolled CommonJS modules. Moreover, to trigger the behaviour in question,
// we want the serializer to think that we are pulling in a dependency from
// node_modules (since local modules will have their objects inlined);
// we thus set up that fake file structure in this test case too.

module.exports.description = "Shadowing reserved identifiers";

exports = require("./node_modules/lib")

module.exports.func = async () => {
    console.log(exports.libFunc.name);
};
