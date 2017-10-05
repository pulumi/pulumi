// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";

class Simple extends pulumi.Resource {
	public readonly b: pulumi.Computed<string>;

	constructor(name: string) {
		let ins: any = { a: "a" };
		let outs: any = { b: "b" };

		super("test:simple:simple", name, {ins: ins, outs: outs}, undefined);
	}
}

async function run(): Promise<void> {
	let s = new Simple("hello-world");
	let s2 = new Simple(await s.b || "<unknown>");

	console.log(await s2.urn);
}

run();
