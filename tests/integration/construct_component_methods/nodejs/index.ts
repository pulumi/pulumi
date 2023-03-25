// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { Component } from "./component";

const component = new Component("component", {
	first: "Hello",
    second: "World",
});

const result = component.getMessage({ name: "Alice" });

export const message = result.message;

export const messagedeps = getDeps(message);

async function getDeps(o: pulumi.Output<string>): Promise<string[]> {
    const resources: Set<pulumi.Resource> = await (message as any).allResources()
    const urns: string[] = [];
    for (const res of resources) {
        urns.push(await (res.urn as any).promise());
    }
    return urns;
}
