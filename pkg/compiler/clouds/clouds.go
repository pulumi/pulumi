// Copyright 2016 Marapongo, Inc. All rights reserved.

package clouds

// Target selects a cloud infrastructure to target when compiling.
type Target int

const (
	AWSTarget    Target = iota // Amazon Web Services
	GCPTarget                  // Google Cloud Platform
	AzureTarget                // Microsoft Azure
	VMWareTarget               // VMWare vSphere, etc.
)

// TargetMap maps human-friendly names to the Targets for those names.
var TargetMap = map[string]Target{
	"aws":    AWSTarget,
	"gcp":    GCPTarget,
	"azure":  AzureTarget,
	"vmware": VMWareTarget,
}
