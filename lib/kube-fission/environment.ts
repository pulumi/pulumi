// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Metadata} from "./metadata";
import * as coconut from "@coconut/coconut";

// Environment identifies the language and OS specific resources that a function depends on.  For now this includes
// only the function run container image.  Later, this will also include build containers, as well as support tools
// like debuggers, profilers, etc.
export class Environment extends coconut.Resource implements EnvironmentProperties {
    public readonly metadata: Metadata;
    public readonly runContainerImageURL: string;

    constructor(args: EnvironmentProperties) {
        super();
        this.metadata = args.metadata;
        this.runContainerImageURL = args.runContainerImageURL;
    }
}

export interface EnvironmentProperties {
    metadata: Metadata;
    runContainerImageURL: string;
}

