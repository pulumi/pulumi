// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import {Error} from "../lib";

export function assert(b: boolean): void {
    if (!b) {
        throw new Error("An assertion has failed");
    }
}

