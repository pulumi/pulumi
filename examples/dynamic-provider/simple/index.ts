// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";
import * as dynamic from "pulumi/dynamic";

class OperatorCallbacks implements dynamic.ResourceProvider {
    private op: (l: number, r: number) => any;

    constructor(op: (l: number, r: number) => any) {
        this.op = op;
    }

    check = (inputs: any) => Promise.resolve(new dynamic.CheckResult(undefined, []));
    diff = (id: pulumi.ID, olds: any, news: any) => Promise.resolve(new dynamic.DiffResult([], []));
    delete = (id: pulumi.ID, props: any) => Promise.resolve();

    create = (inputs: any) => Promise.resolve(new dynamic.CreateResult("0", this.op(Number(inputs.left), Number(inputs.right))));

    update = (id: string, olds: any, news: any) => Promise.resolve(new dynamic.UpdateResult(this.op(Number(news.left), Number(news.right))));
}

class DivCallbacks extends OperatorCallbacks {
    constructor() {
        super((left: number, right: number) => <any>{ quotient: Math.floor(left / right), remainder: left % right });
    }

    check = (ins: any) => Promise.resolve(new dynamic.CheckResult(undefined, ins.right == 0 ? [ new dynamic.CheckFailure("right", "divisor must be non-zero") ] : []));
}

class Add extends dynamic.Resource {
    public readonly sum: pulumi.Computed<number>;

    private static callbacks = new OperatorCallbacks((left: number, right: number) => <any>{ sum: left + right });

    constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
        super(Add.callbacks, name, {left: left, right: right, sum: undefined}, undefined);
    }
}

class Mul extends dynamic.Resource {
    public readonly product: pulumi.Computed<number>;

    private static callbacks = new OperatorCallbacks((left: number, right: number) => <any>{ product: left * right });

    constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
        super(Mul.callbacks, name, {left: left, right: right, product: undefined}, undefined);
    }
}

class Sub extends dynamic.Resource {
    public readonly difference: pulumi.Computed<number>;

    private static callbacks = new OperatorCallbacks((left: number, right: number) => <any>{ difference: left - right });

    constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
        super(Sub.callbacks, name, {left: left, right: right, difference: undefined}, undefined);
    }
}

class Div extends dynamic.Resource {
    public readonly quotient: pulumi.Computed<number>;
    public readonly remainder: pulumi.Computed<number>;

    private static callbacks = new DivCallbacks();

    constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
        super(Div.callbacks, name, {left: left, right: right, quotient: undefined, remainder: undefined}, undefined);
    }
}

let run = async () => {
    let config = new pulumi.Config("simple:config");

    let w = Number(config.require("w")), x = Number(config.require("x")), y = Number(config.require("y"));

    let sum = new Add("sum", x, y);
    let square = new Mul("square", sum.sum, sum.sum);
    let diff = new Sub("diff", square.product, w);
    let divrem = new Div("divrem", diff.difference, sum.sum);
    let result = new Add("result", divrem.quotient, divrem.remainder);

    console.log(`((x + y)^2 - w) / (x + y) + ((x + y)^2 - w) %% (x + y) = ${await result.sum}`);
};

run();
