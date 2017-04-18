// Copyright 2017 Pulumi, Inc. All rights reserved.

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

