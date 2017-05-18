// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"

	"github.com/pulumi/lumi/lib/aws/idl/iam"
)

// The Authorizer resource creates an authorization layer that Amazon API Gateway (API Gateway) activates for
// methods that have authorization enabled. API Gateway activates the authorizer when a client calls those methods.
type Authorizer struct {
	idl.NamedResource
	// Type is the type of authorizer.
	Type AuthorizerType `lumi:"type"`
	// AuthorizerCredentials are the credentials required for the authorizer. To specify an AWS Identity and Access
	// Management (IAM) role that API Gateway assumes, specify the role. To use resource-based permissions on the AWS
	// Lambda (Lambda) function, specify null.
	AuthorizerCredentials *iam.Role `lumi:"authorizerCredentials,optional"`
	// AuthorizerResultTTLInSeconds is the time-to-live (TTL) period, in seconds, that specifies how long API Gateway
	// caches authorizer results.  If you specify a value greater than `0`, API Gateway caches the authorizer responses.
	// By default, API Gateway sets this property to `300`.  The maximum value is `3600`, or 1 hour.
	AuthorizerResultTTLInSeconds *float64 `lumi:"authorizerResultTTLInSeconds,optional"`
	// AuthorizerURI is the authorizer's Uniform Resource Identifier (URI).  If you specify `TOKEN` for the authorizer's
	// type property, specify a Lambda function URI, which has the form `arn:aws:apigateway:region:lambda:path/path`.
	// The path usually has the form `/2015-03-31/functions/LambdaFunctionARN/invocations`.
	AuthorizerURI *string `lumi:"authorizerURI,optional"`
	// IdentitySource is the source of the identity in an incoming request.  If you specify `TOKEN` for the authorizer's
	// type property, specify a mapping expression.  The custom header mapping expression has the form
	// `method.request.header.name`, where name is the name of a custom authorization header that clients submit as part
	// of their requests.
	IdentitySource *string `lumi:"identitySource,optional"`
	// IdentityValidationExpression is a validation expression for the incoming identity.  If you specify `TOKEN` for
	// the authorizer's type property, specify a regular expression.  API Gateway uses the expression to attempt to
	// match the incoming client token, and proceeds if the token matches.  If the token doesn't match, API Gateway
	// responds with a 401 (unauthorized request) error code.
	IdentityValidationExpression *string `lumi:"identityValidationExpression,optional"`
	// providers is a list of the Amazon Cognito user pools to associate with this authorizer.
	Providers *[]idl.Resource/*TODO: cognito.UserPool*/ `lumi:"providers,optional"`
	// RestAPI is the resource in which API Gateway creates the authorizer.
	RestAPI *RestAPI `lumi:"restAPI,optional"`
}

type AuthorizerType string

const (
	TokenAuthorizer   AuthorizerType = "TOKEN"              // a custom authorizer that uses a Lambda function.
	CognitoAuthorizer AuthorizerType = "COGNITO_USER_POOLS" // an authorizer that uses Amazon Cognito user pools.
)
