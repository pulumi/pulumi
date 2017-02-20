// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from '@mu/mu';

// A special service that simply emits a CloudFormation template.
// @name: aws/x/cf
export class Resource
        extends mu.Resource
        implements ResourceProperties {

    public readonly resource: string;
    public readonly properties?: any;
    public readonly dependsOn?: mu.Stack[];

    constructor(args: ResourceProperties) {
        super();
        this.resource = args.resource;
        this.properties = args.properties;
        this.dependsOn = args.dependsOn;
    }
}

export interface ResourceProperties {
    // The CF resource name.
    readonly resource: string;
    // An optional list of properties to map.
    readonly properties?: any /*actually, JSON-like*/;
    // An optional list of other CloudFormation resources that this depends on.
    readonly dependsOn?: mu.Stack[];
}

