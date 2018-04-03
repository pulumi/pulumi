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

import { Provider, Resource } from "./resource";

// Step 4: Fail during an update:
// * Create 1 resource, a4, with a property different than the a3 in Step 3, requiring replacement
//   (CreateReplacement(a4), Update(c3=>c4), DeleteReplaced(a3)).
let a = new Resource("a", { state: 1, replace: 2 });
// * Inject a fault into the Update(c3=>c4), such that we never delete a3 (and it goes onto the checkpoint list).
// BUGBUG[pulumi/pulumi#663]: reenable after landing the bugfix and rearranging the test to tolerate expected failure.
// Provider.instance.injectFault(new Error("intentional update failure during step 4"));
let c = new Resource("c", { state: 1, resource: a });
let e = new Resource("e", { state: 1, resource: c });
// Checkpoint: a4, c3, e3; pending delete: a3
