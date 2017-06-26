// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The APIKey resource creates a unique key that you can distribute to clients who are executing Amazon
// API Gateway (API Gateway) Method resources that require an API key. To specify which API key clients must use, map
// the API key with the RestApi and Stage resources that include the methods requiring a key.
type APIKey struct {
	idl.NamedResource
	// KeyName is a name for the API key. If you don't specify a name, a unique physical ID is generated and used.
	KeyName *string `lumi:"keyName,replaces,optional"`
	// Description is a description of the purpose of the API key.
	Description *string `lumi:"description,optional"`
	// Enabled indicates whether the API key can be used by clients.
	Enabled *bool `lumi:"enabled,optional"`
	// StageKeys is a list of stages to associated with this API key.
	StageKeys *StageKey `lumi:"stageKeys,optional"`
}

type StageKey struct {
	// RestAPI is a RestAPI resource that includes the stage with which you want to associate the API key.
	RestAPI *RestAPI `lumi:"restAPI,optional"`
	// Stage is the stage with which to associate the API key. The stage must be included in the RestAPI
	// resource that you specified in the RestAPI property.
	Stage *Stage `lumi:"stage,optional"`
}
