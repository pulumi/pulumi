// Copyright 2016-2022, Pulumi Corporation.  All rights reserved.
import * as process from "process";
import { Resource } from "./resource";
// Base depends on nothing.
const a = new Resource("base", { uniqueKey: 1, state: 99 });

for(let i = 0; i < 1000; i++) {
    new Resource(`base-${i}`, { uniqueKey: 100+i, state: 99 });
}

// Dependent depends on Base with state 99.
new Resource("dependent", { uniqueKey: a.state.apply(() => {
    if (process.env["PULUMI_NODEJS_DRY_RUN"] != "true") {
        throw Error("`base` should be created and `dependent` should not");
    }
    return 1;
}), state: a.state });
