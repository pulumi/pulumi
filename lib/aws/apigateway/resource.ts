// Copyright 2017 Pulumi, Inc. All rights reserved.

import {RestAPI} from "./restAPI";
import * as cloudformation from "../cloudformation";

// An Amazon API Gateway (API Gateway) API resource.
export class Resource extends cloudformation.Resource implements ResourceProperties {
    public readonly parent: Resource;
    public readonly pathPart: string;
    public readonly restAPI: RestAPI;

    constructor(name: string, args: ResourceProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::Resource",
        });
        this.parent = args.parent;
        this.pathPart = args.pathPart;
        this.restAPI = args.restAPI;
    }
}

export interface ResourceProperties {
    // If you want to create a child resource, the parent resource.  For resources without a parent, specify
    // the RestAPI's root resource.
    readonly parent: Resource;
    // A path name for the resource.
    readonly pathPart: string;
    // The RestAPI resource in which you want to create this resource.
    readonly restAPI: RestAPI;
}

