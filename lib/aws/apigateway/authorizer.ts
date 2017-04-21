// Copyright 2017 Pulumi, Inc. All rights reserved.

import {RestAPI} from "./restAPI";
import * as cloudformation from "../cloudformation";
import * as iam from "../iam";

// The Authorizer resource creates an authorization layer that Amazon API Gateway (API Gateway) activates for
// methods that have authorization enabled. API Gateway activates the authorizer when a client calls those methods.
export class Authorizer extends cloudformation.Resource implements AuthorizerProperties {
    public type: AuthorizerType;
    public authorizerCredentials?: iam.Role;
    public authorizerResultTTLInSeconds?: number;
    public authorizerURI?: string;
    public identitySource?: string;
    public identityValidationExpression?: string;
    public providers?: /*TODO: cognito.UserPool*/any[];
    public restAPI?: RestAPI;

    constructor(name: string, args: AuthorizerProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::Authorizer",
        });
        this.type = args.type;
        this.authorizerCredentials = args.authorizerCredentials;
        this.authorizerResultTTLInSeconds = args.authorizerResultTTLInSeconds;
        this.authorizerURI = args.authorizerURI;
        this.identitySource = args.identitySource;
        this.identityValidationExpression = args.identityValidationExpression;
        this.providers = args.providers;
        this.restAPI = args.restAPI;
    }
}

export interface AuthorizerProperties {
    // type is the type of authorizer.
    type: AuthorizerType;
    // authorizerCredentials are the credentials required for the authorizer. To specify an AWS Identity and Access
    // Management (IAM) role that API Gateway assumes, specify the role. To use resource-based permissions on the AWS
    // Lambda (Lambda) function, specify null.
    authorizerCredentials?: iam.Role;
    // authorizerResultTTLInSeconds is the time-to-live (TTL) period, in seconds, that specifies how long API Gateway
    // caches authorizer results.  If you specify a value greater than `0`, API Gateway caches the authorizer responses.
    // By default, API Gateway sets this property to `300`.  The maximum value is `3600`, or 1 hour.
    authorizerResultTTLInSeconds?: number;
    // authorizerURI is the authorizer's Uniform Resource Identifier (URI).  If you specify `TOKEN` for the authorizer's
    // type property, specify a Lambda function URI, which has the form `arn:aws:apigateway:region:lambda:path/path`.
    // The path usually has the form `/2015-03-31/functions/LambdaFunctionARN/invocations`.
    authorizerURI?: string;
    // identitySource is the source of the identity in an incoming request.  If you specify `TOKEN` for the authorizer's
    // type property, specify a mapping expression.  The custom header mapping expression has the form
    // `method.request.header.name`, where name is the name of a custom authorization header that clients submit as part
    // of their requests.
    identitySource?: string;
    // identityValidationExpression is a validation expression for the incoming identity.  If you specify `TOKEN` for
    // the authorizer's type property, specify a regular expression.  API Gateway uses the expression to attempt to
    // match the incoming client token, and proceeds if the token matches.  If the token doesn't match, API Gateway
    // responds with a 401 (unauthorized request) error code.
    identityValidationExpression?: string;
    // providers is a list of the Amazon Cognito user pools to associate with this authorizer.
    providers?: /*TODO: cognito.UserPool*/any[];
    // restAPI is the resource in which API Gateway creates the authorizer.
    restAPI?: RestAPI;
}

export type AuthorizerType =
    "TOKEN"             | // a custom authorizer that uses a Lambda function.
    "COGNITO_USER_POOLS"; // an authorizer that uses Amazon Cognito user pools.

