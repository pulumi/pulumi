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

// Step 5: Fail during an update:
// * Create 1 resource, a5, with a property different than the a4 in Step 4, requiring replacement
//   (CreateReplacement(a5), Update(c4=>c5), DeleteReplaced(a4)).
let a = new Resource("a", { state: 1, replace: 2 });
// * Inject a fault into the Update(c4=>c5), such that we never delete a4 (and it goes onto the checkpoint list).
// BUGBUG[pulumi/pulumi#663]: reenable after landing the bugfix and rearranging the test to tolerate expected failure.
// Provider.instance.injectFault(new Error("intentional update failure during step 4"));
let c = new Resource("c", { state: 1, replaceDBR: 1, resource: a });
let e = new Resource("e", { state: 1 });
// Checkpoint: a5, c4, e4; pending delete: a4
