// Copyright 2016 Marapongo, Inc. All rights reserved.

package schedulers

// Arch selects a cloud scheduler to target when compiling.
type Arch int

const (
	NoArch         Arch = iota // no scheduler, just use native VMs.
	SwarmArch                  // Docker Swarm
	KubernetesArch             // Kubernetes
	MesosArch                  // Apache Mesos
	ECSArch                    // Amazon Elastic Container Service (only valid for AWS)
	GKEArch                    // Google Container Engine (only valid for GCP)
	ACSArch                    // Microsoft Azure Container Service (only valid for Azure)
)

const (
	noArch         = ""
	swarmArch      = "swarm"
	kubernetesArch = "kubernetes"
	mesosArch      = "mesos"
	ecsArch        = "ecs"
	gkeArch        = "gke"
	acsArch        = "acs"
)

// ArchMap maps human-friendly names to the Archs for those names.
var ArchMap = map[string]Arch{
	noArch:         NoArch,
	swarmArch:      SwarmArch,
	kubernetesArch: KubernetesArch,
	mesosArch:      MesosArch,
	ecsArch:        ECSArch,
	gkeArch:        GKEArch,
	acsArch:        ACSArch,
}

// ArchNames maps Archs to human-friendly names.
var ArchNames = map[Arch]string{
	NoArch:         noArch,
	SwarmArch:      swarmArch,
	KubernetesArch: kubernetesArch,
	MesosArch:      mesosArch,
	ECSArch:        ecsArch,
	GKEArch:        gkeArch,
	ACSArch:        acsArch,
}
