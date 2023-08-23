resource dockerProvider "pulumi:providers:docker" {
	__logicalName = "docker-provider"

	options {
		version = "4.0.0-alpha.0"
	}
}

resource image "docker:index/image:Image" {
	__logicalName = "image"
	imageName = "pulumi.example.com/test-yaml:tag1"
	skipPush = true
	build = {
		"dockerfile" = "Dockerfile",
		"context" = "."
	}

	options {
		version = "4.0.0-alpha.0"
	}
}

output imageName {
	__logicalName = "imageName"
	value = image.imageName
}
