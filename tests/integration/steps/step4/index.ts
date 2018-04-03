// Copyright 2017-2018, Pulumi Corporation.
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

import { Resource } from "./resource";

// Step4: Replace a resource (but this time, deleteBeforeReplace):
// * Create 1 resource, a4, equivalent to the a3 in Step 3 (Same(a3, a4)).
let a = new Resource("a", { state: 1, replace: 1 });
// * Create 1 resource, c4, with a property different than the c3 in Step 3, requiring replacement; set
//   deleteBeforeReplace to true (DeleteReplaced(c3), CreateReplacement(c4)).
let c = new Resource("c", { state: 1, replaceDBR: 1, resource: a });
// * Create 1 resource, e4, equivlaent to the e3 in Step 3 (Same(e3, e4)).
let e = new Resource("e", { state: 1 });
