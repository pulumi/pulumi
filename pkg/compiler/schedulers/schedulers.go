// Copyright 2016 Marapongo, Inc. All rights reserved.

package schedulers

// Target selects a cloud scheduler to target when compiling.
type Target int

const (
	NativeTarget     Target = iota // no scheduler, just use native VMs.
	SwarmTarget                    // Docker Swarm
	KubernetesTarget               // Kubernetes
	MesosTarget                    // Apache Mesos
	ECSTarget                      // Amazon Elastic Container Service (only valid for AWS)
	GKETarget                      // Google Container Engine (only valid for GCP)
	ACSTarget                      // Microsoft Azure Container Service (only valid for Azure)
)

// TargetMap maps human-friendly names to the Targets for those names.
var TargetMap = map[string]Target{
	"":           NativeTarget,
	"swarm":      SwarmTarget,
	"kubernetes": KubernetesTarget,
	"mesos":      MesosTarget,
	"ecs":        ECSTarget,
	"gke":        GKETarget,
	"acs":        ACSTarget,
}
