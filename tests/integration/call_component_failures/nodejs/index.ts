// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import { Component } from "./component"

const component = new Component("testComponent", {
    foo: "bar"
});

const message = component.getMessage({ echo: "hello" }).message;
