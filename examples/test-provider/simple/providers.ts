// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";

export class Operator implements pulumi.testing.ResourceProvider {
    private op: (l: number, r: number) => any;

    constructor(op: (l: number, r: number) => any) {
        this.op = op;
    }

    check(inputs: any): Promise<pulumi.testing.CheckResult> { return Promise.resolve(new pulumi.testing.CheckResult(undefined, [])); }
    diff(id: pulumi.ID, olds: any, news: any): Promise<pulumi.testing.DiffResult> { return Promise.resolve(new pulumi.testing.DiffResult([], [])); }
    delete(id: pulumi.ID, props: any): Promise<void> { return Promise.resolve(); }

    create(inputs: any): Promise<pulumi.testing.CreateResult> {
        return Promise.resolve(new pulumi.testing.CreateResult("0", this.op(Number(inputs.left), Number(inputs.right))));
    }

    update(id: string, olds: any, news: any): Promise<pulumi.testing.UpdateResult> {
        return Promise.resolve(new pulumi.testing.UpdateResult(this.op(Number(news.left), Number(news.right))));
    }
}

export class Div extends Operator {
    constructor() {
        super((left: number, right: number) => <any>{ quotient: Math.floor(left / right), remainder: left % right });
    }

    check(ins: any): Promise<pulumi.testing.CheckResult> {
        return Promise.resolve(new pulumi.testing.CheckResult(undefined, ins.right == 0 ? [ new pulumi.testing.CheckFailure("right", "divisor must be non-zero") ] : []));
    }
}

export var add = new Operator((left: number, right: number) => <any>{ sum: left + right });
export var mul = new Operator((left: number, right: number) => <any>{ product: left * right });
export var sub = new Operator((left: number, right: number) => <any>{ difference: left - right });
export var div = new Div();
