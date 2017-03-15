// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as coconut from '@coconut/coconut';

// A special service that simply emits a CloudFormation template.
// @name: aws/x/cf
export class Resource
        extends coconut.Resource
        implements ResourceProperties {

    public readonly name: string;
    public readonly resource: string;
    public readonly properties?: any;
    public readonly dependsOn?: coconut.Stack[];

    constructor(args: ResourceProperties) {
        super();
        this.name = args.name;
        this.resource = args.resource;
        this.properties = args.properties;
        this.dependsOn = args.dependsOn;
    }
}

export interface ResourceProperties {
    // The resource name.
    readonly name: string;
    // The CF resource name.
    readonly resource: string;
    // An optional list of properties to map.
    readonly properties?: any /*actually, JSON-like*/;
    // An optional list of other CloudFormation resources that this depends on.
    readonly dependsOn?: coconut.Stack[];
}

