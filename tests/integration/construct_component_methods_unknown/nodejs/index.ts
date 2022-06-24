// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import { Component } from "./component"
import { Random } from "./random"

const r = new Random("resource", { length: 10 });
const component = new Component("component");

export const result = component.getMessage({ echo: r.id }).apply(v => {
    console.log("should not run (result)");
    process.exit(1);
});
