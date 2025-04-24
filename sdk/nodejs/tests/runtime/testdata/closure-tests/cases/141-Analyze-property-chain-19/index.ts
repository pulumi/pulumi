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

export const description = "Analyze property chain #19";

const o1 = { c: 2, d: 3 };
const o2 = { a: 1, b: o1 };
const o3 = { a: 1, b: o1 };


export const func = function () { console.log(o2.b.d); console.log(o3.b.d); console.log(o2.b); };
