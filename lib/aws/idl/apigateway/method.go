// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Method resource creates Amazon API Gateway (API Gateway) methods that define the parameters and
// body that clients must send in their requests.
type Method struct {
	idl.NamedResource
	// The HTTP method that clients will use to call this method.
	HTTPMethod string `lumi:"httpMethod"`
	// The API Gateway resource. For root resource methods, specify the RestAPI's root resource ID.
	APIResource *Resource `lumi:"apiResource"`
	// The RestAPI resource in which API Gateway creates the method.
	RestAPI *RestAPI `lumi:"restAPI"`
	// Indicates whether the method requires clients to submit a valid API key.
	APIKeyRequired *bool `lumi:"apiKeyRequired,optional"`
	// The method's authorization type.  Required to be "CUSTOM" if you specify an authorizer.
	AuthorizationType *AuthorizationType `lumi:"authorizationType,optional"`
	// The authorizer to use on this method.  If you specify this, make sure authorizationType is set to "CUSTOM".
	Authorizer *Authorizer `lumi:"authorizer,optional"`
	// The back-end system that the method calls when it receives a request.
	Integration *Integration `lumi:"integration,optional"`
	// The responses that can be sent to the client who calls the method.
	MethodResponses *[]MethodResponse `lumi:"methodResponses,optional"`
	// The resources used for the response's content type.  Specify response models as key-value pairs, with a content
	// type (string) as the key and a Model resource as the value.
	RequestModels *map[string]*Model `lumi:"requestModels,optional"`
	// Request parameters that API Gateway accepts.  Specify request parameters as key-value pairs (string-to-Boolean
	// map), with a source as the key and a Boolean as the value.  The Boolean specifies whether a parameter is
	// required.  A source must match the following format `method.request.location.name`, where the `location` is
	// `querystring`, `path`, or `header`, and `name` is a valid, unique parameter name.
	RequestParameters *map[string]bool `lumi:"requestParameters,optional"`
}

// The method's authorization type.
type AuthorizationType string

const (
	NoAuthorization      AuthorizationType = "NONE"               // open access.
	AWSIAMAuthorization  AuthorizationType = "AWS_IAM"            // using AWS IAM permissions.
	CustomAuthorization  AuthorizationType = "CUSTOM"             // a custom authorizer.
	CognitoAuthorization AuthorizationType = "COGNITO_USER_POOLS" // a Cognito user pool.
)

// Integration specifies information about the target back end that an Amazon API Gateway method calls.
type Integration struct {
	// The type of back end your method is running.
	Type IntegrationType `lumi:"type"`
	// A list of request parameters whose values API Gateway will cache.
	CacheKeyParameters *[]string `lumi:"cacheKeyParameters,optional"`
	// An API-specific tag group of related cached parameters.
	CacheNamespace *string `lumi:"cacheNamespace,optional"`
	// The credentials required for the integration.  To specify an AWS Identity and Access Management (IAM) role that
	// API Gateway assumes, specify the role's Amazon Resource Name (ARN).  To require that the caller's identity be
	// passed through from the request, specify arn:aws:iam::*:user/*.
	//
	// To use resource-based permissions on the AWS Lambda (Lambda) function, don't specify this property. Use the
	// AWS::Lambda::Permission resource to permit API Gateway to call the function.  For more information, see
	// http://docs.aws.amazon.com/lambda/latest/dg/access-control-resource-based.html#access-control-resource-based-example-apigateway-invoke-function.
	Credentials *string `lumi:"credentials,optional"`
	// The integration's HTTP method type.  This is required for all types except for "MOCK".
	IntegrationHTTPMethod *string `lumi:"integrationHTTPMethod,optional"`
	// The response that API Gateway provides after a method's back end completes processing a request.  API Gateway
	// intercepts the back end's response so that you can control how API Gateway surfaces back-end responses.  For
	// example, you can map the back-end status codes to codes that you define.
	IntegrationResponse *[]IntegrationResponse `lumi:"integrationResponse,optional"`
	// Indicates when API Gateway passes requests to the targeted back end.  This behavior depends on the request's
	// Content-Type header and whether you defined a mapping template for it.
	PassthroughBehavior *PassthroughBehavior `lumi:"passthroughBehavior,optional"`
	// The request parameters that API Gateway sends with the back-end request.  Specify request parameters as key-value
	// pairs (string-to-string maps), with a destination as the key and a source as the value.
	//
	// Specify the destination using the following pattern `integration.request.location.name`, where `location` is
	// `querystring`, `path`, or `header`, and `name` is a valid, unique parameter name.
	//
	// The source must be an existing method request parameter or a static value.  Static values must be enclosed in
	// single quotation marks and pre-encoded based on their destination in the request.
	RequestParameters *map[string]string `lumi:"requestParameters,optional"`
	// A map of Apache Velocity templates that are applied on the request payload.  The template that API Gateway uses
	// is based on the value of the Content-Type header sent by the client. The content type value is the key, and the
	// template is the value (specified as a string).  For more information about templates, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-mapping-template-reference.html.
	RequestTemplates *map[string]string `lumi:"requestTemplates,optional"`
	// The integration's Uniform Resource Identifier (URI).
	//
	// If you specify "HTTP" for the Type property, specify the API endpoint URL.
	//
	// If you specify "MOCK" for the Type property, don't specify this property.
	//
	// If you specify "AWS" for the Type property, specify an AWS service that follows the form:
	// `arn:aws:apigateway:region:subdomain.service|service:path|action/service_api`.  For example, a Lambda function
	// URI follows the form: `arn:aws:apigateway:region:lambda:path/path`.  The path is usually in the form
	// `/2015-03-31/functions/LambdaFunctionARN/invocations`.  For more information, see Integration's URI property.
	//
	// If you specify "HTTP" or "AWS" for the Type property, you must specify the URI property.
	URI *string `lumi:"uri,optional"`
}

// IntegrationType specifies an Integration's type.
type IntegrationType string

const (
	HTTPIntegration      IntegrationType = "HTTP"       // for integrating with an HTTP back end.
	HTTPProxyIntegration IntegrationType = "HTTP_PROXY" // for integrating with the HTTP proxy integration.
	AWSIntegration       IntegrationType = "AWS"        // for any AWS service endpoints.
	AWSProxyIntegration  IntegrationType = "AWS_PROXY"  // for integrating with the Lambda proxy integration type.
	MockIntegration      IntegrationType = "MOCK"       // for testing without actually invoking the back end.
)

// IntegrationResponse specifies the response that Amazon API Gateway sends after a method's back end finishes
// processing a request.
type IntegrationResponse struct {
	// The response parameters from the back-end response that API Gateway sends to the method response. Specify
	// response parameters as key-value pairs (string-to-string mappings).
	//
	// Use the destination as the key and the source as the value:
	//
	//     * The destination must be an existing response parameter in the MethodResponse property.
	//
	//     * The source must be an existing method request parameter or a static value. You must enclose static values
	//       in single quotation marks and pre-encode these values based on the destination specified in the request.
	//
	// For more information, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/request-response-data-mappings.html.
	ResponseParameters *map[string]string `lumi:"responseParameters,optional"`
	// The templates used to transform the integration response body.  Specify templates as key-value pairs
	// (string-to-string maps), with a content type as the key and a template as the value.  For more information, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-mapping-template-reference.html.
	ResponseTemplates *map[string]string `lumi:"responseTemplates,optional"`
	// A regular expression that specifies which error strings or status codes from the back end map to the integration
	// response.
	SelectionPattern *string `lumi:"selectionPattern,optional"`
	// The status code that API Gateway uses to map the integration response to a MethodResponse status code.
	StatusCode *string `lumi:"statusCode,optional"`
}

// PassthroughBehavior specifies how the method request body of an unmapped content type will be passed through the
// integration request to the back end without transformation.  A content type is unmapped if no mapping template is
// defined in the integration or the content type does not match any of the mapped content types.
type PassthroughBehavior string

const (
	// Passes the method request body through the integration request to the back end without transformation when the
	// method request content type does not match any content type associated with the mapping templates defined in the
	// integration request.
	PassthroughWhenNoMatch PassthroughBehavior = "WHEN_NO_MATCH"
	// Passes the method request body through the integration request to the back end without transformation when no
	// mapping template is defined in the integration request.  If a template is defined when this option is selected,
	// the method request of an unmapped content-type will be rejected with an HTTP 415 Unsupported Media Type response.
	PassthroughWhenNoTemplates PassthroughBehavior = "WHEN_NO_TEMPLATES"
	// Rejects the method request with an HTTP 415 Unsupported Media Type response when either the method request
	// content type does not match any content type associated with the mapping templates defined in the integration
	// request or no mapping template is defined in the integration request.
	PassthroughNever PassthroughBehavior = "NEVER"
)

// MethodResponse defines the responses that can be sent to the client who calls an Amazon API Gateway method.
type MethodResponse struct {
	// The method response's status code, which you map to an IntegrationResponse.
	StatusCode string `lumi:"statusCode"`
	// The resources used for the response's content type.  Specify response models as key-value pairs, with a content
	// type as the key (string) and a Model resource as the value.
	ResponseModels *map[string]*Model `lumi:"responseModels,optional"`
	// Response parameters that API Gateway sends to the client that called a method.  Specify response parameters as
	// key-value pairs (string-to-Boolean maps), with a destination as the key and a Boolean as the value.  Specify the
	// destination using the following pattern: `method.response.header.name`, where the `name` is a valid, unique
	// header name.  The Boolean specifies whether a parameter is required.
	ResponseParameters *map[string]bool `lumi:"responseParameters,optional"`
}

// MethodSetting configures settings for all methods in an Amazon API Gateway (API Gateway) stage.
type MethodSetting struct {
	// Indicates whether the cached responses are encrypted.
	CacheDataEncrypted *bool `lumi:"cacheDataEncrypted,optional"`
	// The time-to-live (TTL) period, in seconds, that specifies how long API Gateway caches responses.
	CacheTTLInSeconds *float64 `lumi:"cacheTTLInSeconds,optional"`
	// Indicates whether responses are cached and returned for requests. You must enable a cache cluster on the stage
	// to cache responses. For more information, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-caching.html.
	CachingEnabled *bool `lumi:"cachingEnabled,optional"`
	// Indicates whether data trace logging is enabled for methods in the stage. API Gateway pushes these logs to Amazon
	// CloudWatch Logs.
	DataTraceEnabled *bool `lumi:"dataTraceEnabled,optional"`
	// The HTTP method.
	HTTPMethod *string `lumi:"httpMethod,optional"`
	// The logging level for this method.
	LoggingLevel *LoggingLevel `lumi:"loggingLevel,optional"`
	// Indicates whether Amazon CloudWatch metrics are enabled for methods in the stage.
	MetricsEnabled *bool `lumi:"metricsEnabled,optional"`
	// The resource path for this method.  Forward slashes (`/`) are encoded as `~1` and the initial slash must include
	// a forward slash.  For example, the path value `/resource/subresource` must be encoded as
	// `/~1resource~1subresource.`  To specify the root path, use only a slash (`/`).
	ResourcePath *string `lumi:"resourcePath,optional"`
	// The number of burst requests per second that API Gateway permits across all APIs, stages, and methods in your
	// AWS account. For more information, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
	ThrottlingBurstLimit *float64 `lumi:"throttlingBurstLimit,optional"`
	// The number of steady-state requests per second that API Gateway permits across all APIs, stages, and methods in
	// your AWS account. For more information, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
	ThrottlingRateLimit *float64 `lumi:"throttlingRateLimit,optional"`
}

// Specifies the logging level for this method, which effects the log entries pushed to Amazon CloudWatch Logs.
type LoggingLevel string

const (
	LoggingOff        LoggingLevel = "OFF"
	LoggingErrorLevel LoggingLevel = "ERROR"
	LoggingInfoLevel  LoggingLevel = "INFO"
)
