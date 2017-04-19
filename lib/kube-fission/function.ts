// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Environment} from "./environment";
import {Metadata} from "./metadata";
import * as coconut from "@coconut/coconut";

// Function is a unit of executable code.  Though it's called a function, the code may have more than one function;
// it's usually some sort of module or package.
export class Function extends coconut.Resource implements FunctionProperties {
    public readonly metadata: Metadata;
    public readonly environment: Environment;
    public readonly code: coconut.asset.Asset;

    constructor(args: FunctionProperties) {
        super();
        this.metadata = args.metadata;
        this.environment = args.environment;
        this.code = args.code;
    }
}

export interface FunctionProperties {
    metadata: Metadata;
    environment: Environment;
    code: coconut.asset.Asset;
}

