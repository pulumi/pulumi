// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";
import * as dynamic from "pulumi/dynamic";

const sleep = require("sleep-promise");
const assert = require("assert");

class NullProvider implements dynamic.ResourceProvider {
    check = (olds: any, news: any) => Promise.resolve({ inputs: news });
    diff = (id: pulumi.ID, olds: any, news: any) => Promise.resolve({});
    create = (inputs: any) => Promise.resolve({ id: "0" });
    update = (id: string, olds: any, news: any) => Promise.resolve({});
    delete = (id: pulumi.ID, props: any) => Promise.resolve();
}

class NullResource extends dynamic.Resource {
    constructor(name: string, input: pulumi.ComputedValue<string>) {
        super(new NullProvider(), name, {input: input}, undefined);
    }
}

async function getInput(): pulumi.Computed<string> {
    await sleep(1000);

    return (new NullResource("a", "")).urn;
}

const b = new NullResource("b", getInput());
