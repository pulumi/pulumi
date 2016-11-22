// Copyright 2016 Marapongo, Inc. All rights reserved.

package schedulers

//  selects a cloud scheduler to target when compiling.
type Arch int

const (
	None        Arch = iota // no scheduler, just use native VMs.
	DockerSwarm             // Docker Swarm
	Kubernetes              // Kubernetes
	Mesos                   // Apache Mesos
	AWSECS                  // Amazon Elastic Container Service (only valid for AWS)
	GCPGKE                  // Google Container Engine (only valid for GCP)
	AzureACS                // Microsoft Azure Container Service (only valid for Azure)
)

const (
	none        = ""
	dockerSwarm = "swarm"
	kubernetes  = "kubernetes"
	mesos       = "mesos"
	awsECS      = "ecs"
	gcpGKE      = "gke"
	azureACS    = "acs"
)

// Names maps s to human-friendly names.
var Names = map[Arch]string{
	None:        none,
	DockerSwarm: dockerSwarm,
	Kubernetes:  kubernetes,
	Mesos:       mesos,
	AWSECS:      awsECS,
	GCPGKE:      gcpGKE,
	AzureACS:    azureACS,
}

// Values maps human-friendly names to the s for those names.
var Values = map[string]Arch{
	none:        None,
	dockerSwarm: DockerSwarm,
	kubernetes:  Kubernetes,
	mesos:       Mesos,
	awsECS:      AWSECS,
	gcpGKE:      GCPGKE,
	azureACS:    AzureACS,
}
