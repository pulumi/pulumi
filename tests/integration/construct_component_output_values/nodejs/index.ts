// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

import { Component } from "./component";

new Component("component", {
    foo: {
        something: "hello",
    },
    bar: {
        tags: {
            "a": "world",
            "b": pulumi.secret("shh"),
        },
    },
});
