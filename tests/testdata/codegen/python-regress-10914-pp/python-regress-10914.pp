config instanceType string {
    default = "t2.micro"
}

ami = invoke("aws:ec2/getAmi:getAmi", {
    filters = [{
        name = "name",
        values = ["amzn-ami-hvm-*"]
    }],
    owners = ["137112412989"],
    mostRecent = true
}).id

userData = "#!/bin/bash\necho \"Hello, World from Pulumi!\" > index.html\nnohup python -m SimpleHTTPServer 80 &"

resource secGroup "aws:ec2/securityGroup:SecurityGroup" {
    __logicalName = "secGroup"
    description = "Enable HTTP access"
    ingress = [{
        fromPort = 80,
        toPort = 80,
        protocol = "tcp",
        cidrBlocks = ["0.0.0.0/0"]
    }]
    tags = {
        "Name" = "web-secgrp"
    }
}

resource server "aws:ec2/instance:Instance" {
    __logicalName = "server"
    instanceType = instanceType
    vpcSecurityGroupIds = [secGroup.id]
    userData = userData
    ami = ami
    tags = {
        "Name" = "web-server-www"
    }
}

output publicIP {
    __logicalName = "publicIP"
    value = server.publicIp
}

output publicDNS {
    __logicalName = "publicDNS"
    value = server.publicDns
}