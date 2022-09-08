resource provider "pulumi:providers:kubernetes" {
	__logicalName = "provider"
	enableServerSideApply = true
}

resource cluster "eks:index:Cluster" {
	__logicalName = "cluster"
	version = 1.21

	options {
		provider = provider
	}
}

resource patch1 "kubernetes:apps/v1:DeploymentPatch" {
	__logicalName = "patch1"
	metadata = {
		name = "coredns",
		namespace = "kube-system",
		annotations = {
			"pulumi.com/patchForce" = true
		}
	}
	spec = {
		template = {
			containers = {
				"name" = "coredns",
				"image" = "602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/coredns:v1.8.7-eksbuild.1"
			}
		}
	}

	options {
		provider = provider
	}
}
