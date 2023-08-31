// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import { Component } from "./component";
import { TestProvider } from "./testProvider";

const testProvider = new TestProvider("testProvider");

const component1 = new Component("component1", {
	first: "Hello",
    second: "World",
}, { provider: testProvider });

const result1 = component1.getMessage({ name: "Alice" });

const component2 = new Component("component2", {
	first: "Hi",
    second: "There",
}, { providers: [testProvider] });

const result2 = component2.getMessage({ name: "Bob" });

export const message1 = result1.message;
export const message2 = result2.message;
