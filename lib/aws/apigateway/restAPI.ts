// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as cloudformation from "../cloudformation";
import * as s3 from "../s3";

// The RestAPI resource contains a collection of Amazon API Gateway (API Gateway) resources and methods that can be
// invoked through HTTPS endpoints.
export class RestAPI extends cloudformation.Resource implements RestAPIProperties {
    public body?: any;
    public bodyS3Location?: S3Location;
    public cloneFrom?: RestAPI;
    public description?: string;
    public failOnWarnings?: boolean;
    public apiName?: string;
    public parameters?: string[];

    constructor(name: string, args: RestAPIProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::RestApi",
        });
        this.body = args.body;
        this.bodyS3Location = args.bodyS3Location;
        this.cloneFrom = args.cloneFrom;
        this.description = args.description;
        this.failOnWarnings = args.failOnWarnings;
        this.apiName = args.apiName;
        this.parameters = args.parameters;
    }
}

export interface RestAPIProperties {
    // An OpenAPI specification that defines a set of RESTful APIs in the JSON format.
    body?: any;
    // The Amazon Simple Storage Service (Amazon S3) location that points to a OpenAPI file, which defines a set of
    // RESTful APIs in JSON or YAML format.
    bodyS3Location?: S3Location;
    // Another API Gateway RestAPI resource that you want to clone.
    cloneFrom?: RestAPI;
    // A description of the purpose of this API Gateway RestAPI resource.
    description?: string;
    // If a warning occurs while API Gateway is creating the RestAPI resource, indicates whether to roll back the
    // resource.
    failOnWarnings?: boolean;
    // A name for the API Gateway RestApi resource.  Required if you don't specify an OpenAPI definition.
    apiName?: string;
    // Custom header parameters for the request.
    parameters?: string[];
}

// S3Location is a property of the RestAPI resource that specifies the Amazon Simple Storage Service (Amazon S3)
// location of a OpenAPI (formerly Swagger) file that defines a set of RESTful APIs in JSON or YAML.
export interface S3Location {
    // The S3 object corresponding to the OpenAPI file.
    object: s3.Object;
    // The Amazon S3 ETag (a file checksum) of the OpenAPI file.  If you don't specify a value, API Gateway skips ETag
    // validation of your OpenAPI file.
    etag?: string;
    // For versioning-enabled buckets, a specific version of the OpenAPI file.
    version?: string;
}

