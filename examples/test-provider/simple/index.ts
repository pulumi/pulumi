// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";

class Add extends pulumi.ExternalResource {
    public readonly sum: pulumi.Computed<number>;

    constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
        super("testing:simple:add", name, {left: left, right: right, sum: undefined}, undefined);
    }
}

class Mul extends pulumi.ExternalResource {
    public readonly product: pulumi.Computed<number>;

    constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
        super("testing:simple:mul", name, {left: left, right: right, product: undefined}, undefined);
    }
}

class Sub extends pulumi.ExternalResource {
    public readonly difference: pulumi.Computed<number>;

    constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
        super("testing:simple:sub", name, {left: left, right: right, difference: undefined}, undefined);
    }
}

class Div extends pulumi.ExternalResource {
    public readonly quotient: pulumi.Computed<number>;
    public readonly remainder: pulumi.Computed<number>;

    constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
        super("testing:simple:div", name, {left: left, right: right, quotient: undefined, remainder: undefined}, undefined);
    }
}

let config = new pulumi.Config("simple:config");

let w = Number(config.require("w")), x = Number(config.require("x")), y = Number(config.require("y"));

let sum = new Add("sum", x, y);
let square = new Mul("square", sum.sum, sum.sum);
let diff = new Sub("diff", square.product, w);
let divrem = new Div("divrem", diff.difference, sum.sum);
let result = new Add("result", divrem.quotient, divrem.remainder);

let output = async function(): Promise<void> {
    console.log(`((x + y)^2 - w) / (x + y) + ((x + y)^2 - w) %% (x + y) = ${await result.sum}`);
};

output();
