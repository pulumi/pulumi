// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as resource from "../resource";

export class CheckResult {
    public readonly defaults: any | undefined;
    public readonly failures: CheckFailure[];

    constructor(defaults: any | undefined, failures: CheckFailure[]) {
        this.defaults = defaults;
        this.failures = failures;
    }
}

export class CheckFailure {
    public readonly property: string;
    public readonly reason: string;

    constructor(property: string, reason: string) {
        this.property = property;
        this.reason = reason;
    }
}

export class DiffResult {
    public readonly replaces: string[];

    constructor(replaces: string[]) {
        this.replaces = replaces;
    }
}

export class CreateResult {
    public readonly id: resource.ID;
    public readonly outs: any | undefined;

    constructor(id: resource.ID, outs: any | undefined) {
        this.id = id;
        this.outs = outs;
    }
}

export class UpdateResult {
    public readonly outs: any | undefined;

    constructor(outs: any | undefined) {
        this.outs = outs;
    }
}

export interface Provider {
    check(inputs: any): Promise<CheckResult>;
    diff(id: resource.ID, olds: any, news: any): Promise<DiffResult>;
    create(inputs: any): Promise<CreateResult>;
    update(id: resource.ID, olds: any, news: any): Promise<UpdateResult>;
    delete(id: resource.ID, props: any): Promise<void>;
}
