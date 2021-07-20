// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import { Component } from "./component";

const component = new Component("component", {
	first: "Hello",
    second: "World",
});

const result = component.getMessage({ name: "Alice" });

export const message = result.message;
