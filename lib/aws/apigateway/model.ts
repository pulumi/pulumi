// Copyright 2017 Pulumi, Inc. All rights reserved.

import {RestAPI} from "./restAPI";
import * as cloudformation from "../cloudformation";

// The Model resource defines the structure of a request or response payload for an Amazon API Gateway method.
export class Model extends cloudformation.Resource implements ModelProperties {
    public readonly contentType: string;
    public readonly restAPI: RestAPI;
    public schema: any;
    public readonly modelName?: string;
    public description?: string;

    constructor(name: string, args: ModelProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::Model",
        });
        this.contentType = args.contentType;
        this.restAPI = args.restAPI;
        this.schema = args.schema;
        this.modelName = args.modelName;
        this.description = args.description;
    }
}

export interface ModelProperties {
    // The content type for the model.
    readonly contentType: string;
    // The REST API with which to associate this model.
    readonly restAPI: RestAPI;
    // The schema to use to transform data to one or more output formats. Specify null (`{}`) if you don't want to
    // specify a schema.
    schema: any;
    // A name for the model.  If you don't specify a name, a unique physical ID is generated and used.
    readonly modelName?: string;
    // A description that identifies this model.
    description?: string;
}

