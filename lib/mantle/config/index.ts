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

import * as arch from "../arch";

export let cloud: arch.Cloud | undefined; // the cloud to target.
export let scheduler: arch.Scheduler | undefined; // the scheduler to target.

// requireArch fetches the target cloud and container scheduler architecture.
export function requireArch(): arch.Arch {
    if (cloud === undefined) {
        throw new Error("No cloud target has been configured (`mantle:config:cloud`)");
    }
    return {
        cloud: cloud,
        scheduler: scheduler,
    };
}

