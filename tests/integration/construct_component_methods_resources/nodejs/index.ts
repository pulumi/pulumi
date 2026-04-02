// Copyright 2016, Pulumi Corporation.  All rights reserved.

import { Component } from "./component";

const component = new Component("component");

export const result = component.createRandom({ length: 10 }).result;
