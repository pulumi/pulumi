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

// Step 1: Populate the world:
// * Create 4 resources, a1, b1, c1, d1.  c1 depends on a1 via an ID property.
let a = new Resource("a", { state: 1 });
let b = new Resource("b", { state: 1, resource: a });
let c = new Resource("c", { state: 1, resource: b });
let d = new Resource("d", { state: 1, resource: c });
// Checkpoint: a1, b1, c1, d1
