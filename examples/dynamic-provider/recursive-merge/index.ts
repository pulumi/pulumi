// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";
import * as dynamic from "pulumi/dynamic";

class TestProvider implements dynamic.ResourceProvider {
    diff = (id: pulumi.ID, olds: any, news: any) => Promise.resolve(new dynamic.DiffResult([], []));
    delete = (id: pulumi.ID, props: any) => Promise.resolve();
    update = (id: string, olds: any, news: any) => Promise.resolve(new dynamic.UpdateResult({}));

    check = (inputs: any) => Promise.resolve(new dynamic.CheckResult({prop: {def: true}, arr: [{def: true}], arr1: [], arr2: [{def:true}]}, []));

    create = async (inputs: any) => {
        if (!inputs.prop.def || !inputs.arr[0].def || !inputs.arr1[0].def || !inputs.arr2[0].def) {
            throw new Error("expected defaults to be recursively merged");
        }
        return new dynamic.CreateResult("0", {ok: true});
    };
}

class TestResource extends dynamic.Resource {
    public readonly ok: pulumi.Computed<Boolean>;

    constructor(name: string) {
        super(new TestProvider(), name, {prop: {unused: ""}, arr: [{unused: ""}], arr1: [{def: true}], arr2: []}, undefined);
    }
}

export const ok = new TestResource("test").ok;
