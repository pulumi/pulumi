// Copyright 2016 Marapongo, Inc. All rights reserved.

package clouds

// Arch selects a cloud infrastructure to target when compiling.
type Arch int

const (
	None   Arch = iota // no target specified.
	AWS                // Amazon Web Services.
	GCP                // Google Cloud Platform.
	Azure              // Microsoft Azure.
	VMWare             // VMWare vSphere, etc.
)

const (
	none   = ""
	aws    = "aws"
	gcp    = "gcp"
	azure  = "azure"
	vmware = "vmware"
)

// Names maps Archs to human-friendly names.
var Names = map[Arch]string{
	None:   none,
	AWS:    aws,
	GCP:    gcp,
	Azure:  azure,
	VMWare: vmware,
}

// Values maps human-friendly names to the Archs for those names.
var Values = map[string]Arch{
	none:   None,
	aws:    AWS,
	gcp:    GCP,
	azure:  Azure,
	vmware: VMWare,
}
