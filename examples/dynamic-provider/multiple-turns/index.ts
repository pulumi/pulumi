// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";
import * as dynamic from "pulumi/dynamic";

const sleep = require("sleep-promise");
const assert = require("assert");

class NullProvider implements dynamic.ResourceProvider {
    check = (inputs: any) => Promise.resolve(new dynamic.CheckResult(undefined, []));
    diff = (id: pulumi.ID, olds: any, news: any) => Promise.resolve(new dynamic.DiffResult([], []));
    create = (inputs: any) => Promise.resolve(new dynamic.CreateResult("0", {}));
    update = (id: string, olds: any, news: any) => Promise.resolve(new dynamic.UpdateResult({}));
    delete = (id: pulumi.ID, props: any) => Promise.resolve();
}

class NullResource extends dynamic.Resource {
    constructor(name: string) {
        super(new NullProvider(), name, {}, undefined);
    }
}

(async () => {
    try {
        const a = new NullResource("a");
        await sleep(1000);
        const b = new NullResource("b");
        const urn = await b.urn;
        assert.notStrictEqual(urn, undefined, "expected a defined urn");
        assert.notStrictEqual(urn, "", "expected a valid urn");
    } catch (err) {
        console.error(err);
        process.exit(-1);
    }
})();
