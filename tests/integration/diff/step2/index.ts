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

// Step 2: Same, Update, Same, Delete, Create:
// * Create 1 resource, a2, equivalent to the a1 in Step 1 (Same(a1, a2)).
let a = new Resource("a", { state: 1 });
// * Create 1 resource, b2, with a property different than the b1 in Step 1 (Update(b1=>b2)).
let b = new Resource("b", { state: 2, resource: a });
// * Create 1 resource, c2, equivalent to the c1 in Step 1 (Same(c1, c2)).
let c = new Resource("c", { state: 1, resource: b });
// * Elide d (Delete(d1)).
// * Create 1 resource, e2, not present in Step 1 (Create(e2)).
let e = new Resource("e", { state: 1, resource: c });
// Checkpoint: a2, b2, c2, e2
