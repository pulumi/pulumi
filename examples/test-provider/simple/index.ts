// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";

class Sum extends pulumi.Resource {
	public readonly sum: pulumi.Computed<number>;

	constructor(name: string, left: pulumi.ComputedValue<number>, right: pulumi.ComputedValue<number>) {
		super("test:provider:sum", name, {left: left, right: right, sum: undefined}, undefined);
	}
}

let config = new pulumi.Config("simple:config");

let x = Number(config.require("x")), y = Number(config.require("y"));
let v = Number(config.require("v")), w = Number(config.require("w"));

let left = new Sum("left", x, y);
let right = new Sum("right", v, w);
let total = new Sum("total", left.sum, right.sum);
