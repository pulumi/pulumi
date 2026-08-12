// Copyright 2016, Pulumi Corporation.  All rights reserved.

import { Component } from "./component"

const component = new Component("testComponent", {
    foo: "bar"
});

const message = component.getMessage({ echo: "hello" }).message;
