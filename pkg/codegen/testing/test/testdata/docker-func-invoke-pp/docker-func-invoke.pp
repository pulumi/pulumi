latest = invoke("docker:index/getRemoteImage:getRemoteImage", {
	name = "nginx"
})

resource ubuntu "docker:index/remoteImage:RemoteImage" {
	__logicalName = "ubuntu"
	name = "ubuntu:precise"
}

output remoteImageId {
	__logicalName = "remoteImageId"
	value = latest.id
}

output ubuntuImage {
	__logicalName = "ubuntuImage"
	value = ubuntu.name
}
