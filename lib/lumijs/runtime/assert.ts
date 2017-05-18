// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Error} from "../lib";

export function assert(b: boolean): void {
    if (!b) {
        throw new Error("An assertion has failed");
    }
}

