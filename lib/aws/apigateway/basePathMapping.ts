// Copyright 2017 Pulumi, Inc. All rights reserved.

import {RestAPI} from "./restAPI";
import {Stage} from "./stage";
import * as cloudformation from "../cloudformation";

// The BasePathMapping resource creates a base path that clients who call your Amazon API Gateway API
// must use in the invocation URL.
export class BasePathMapping extends cloudformation.Resource implements BasePathMappingProperties {
    public domainName: string;
    public restAPI: RestAPI;
    public basePath?: string;
    public stage?: Stage;

    constructor(name: string, args: BasePathMappingProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::BasePathMapping",
        });
        this.domainName = args.domainName;
        this.restAPI = args.restAPI;
        this.basePath = args.basePath;
        this.stage = args.stage;
    }
}

export interface BasePathMappingProperties {
    // domainName is the domain name for the base path mapping.
    domainName: string;
    // restAPI is the API to map.
    restAPI: RestAPI;
    // basePath is the base path that callers of the API must provider in the URL after the domain name.
    basePath?: string;
    // stage is the mapping's API stage.
    stage?: Stage;
}

