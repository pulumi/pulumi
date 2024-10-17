// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import { Component } from "./component"

const component = new Component("component");
const result = component.getMessage({ echo: "hello" });
export const message = result.message;
