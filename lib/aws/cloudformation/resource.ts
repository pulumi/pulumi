// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from '@mu/mu';

// A special service that simply emits a CloudFormation template.
// @name: aws/x/cf
export class Resource extends mu.Resource {
    private args: ResourceArgs;
    constructor(args: ResourceArgs) {
        super();
        // TODO: encode the special translation logic as code (maybe as an overridden method).
        this.args = args;
    }
}

export interface ResourceArgs {
    // The CF resource name.
    readonly resource: string;
    // An optional list of properties to map.
    readonly properties?: any /*actually, JSON-like*/;
    // An optional list of other CloudFormation resources that this depends on.
    readonly dependsOn?: mu.Stack[];
}

