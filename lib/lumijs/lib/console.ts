// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import {printf} from "@lumi/lumirt"

export class Console {
    log(message: any) {
        printf(message);
        printf("\n");
    }
}

export let console = new Console();
