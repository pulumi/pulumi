// Copyright 2017 Pulumi, Inc. All rights reserved.

import {RestAPI} from "./restAPI";
import {Stage} from "./stage";
import * as cloudformation from "../cloudformation";

// The APIKey resource creates a unique key that you can distribute to clients who are executing Amazon
// API Gateway (API Gateway) Method resources that require an API key. To specify which API key clients must use, map
// the API key with the RestApi and Stage resources that include the methods requiring a key.
export class APIKey extends cloudformation.Resource implements APIKeyProperties {
    public readonly keyName?: string;
    public description?: string;
    public enabled?: boolean;
    public stageKeys?: StageKey;

    constructor(name: string, args: APIKeyProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::ApiKey",
        });
        this.keyName = args.keyName;
        this.description = args.description;
        this.enabled = args.enabled;
        this.stageKeys = args.stageKeys;
    }
}

export interface APIKeyProperties {
    // keyName is a name for the API key. If you don't specify a name, a unique physical ID is generated and used.
    readonly keyName?: string;
    // description is a description of the purpose of the API key.
    description?: string;
    // enabled indicates whether the API key can be used by clients.
    enabled?: boolean;
    // stageKeys is a list of stages to associated with this API key.
    stageKeys?: StageKey;
}

export interface StageKey {
    // restAPI is a RestAPI resource that includes the stage with which you want to associate the API key.
    restAPI?: RestAPI;
    // stage is the stage with which to associate the API key. The stage must be included in the RestAPI
    // resource that you specified in the RestAPI property.
    stage?: Stage;
}

