resource explicitProvider "pulumi:providers:aws" {
    region = "us-west-2"
}

zone = invoke("aws:index:getAvailabilityZones", {
    allAvailabilityZones = true
}, {
    provider = explicitProvider
    parent = explicitProvider
    version = "1.2.3"
    pluginDownloadUrl = "https://example.com"
})

resource server "aws:ec2:Instance" {
	instanceType = "t2.micro"
}

zoneWithDepends = invoke("aws:index:getAvailabilityZones", {
    allAvailabilityZones = true
}, {
    dependsOn: [server]
})
