config "vpcTag" "string" {
    default = null
    description = "The tag of the VPC"
}

config "vpcId" "string" {
    default = null
    description = "The id of a VPC to use instead of creating a new one"
}

config "subnets" "list(string)" {
    default = null
    description = "The list of subnets to use"
}

config moreTags "map(string)" {
    default = null
    description = "Additional tags to add to the VPC"
}

config userdata "object({path=string, content=string})" {
    default = null
    description = "The userdata to use for the instances"
}

config complexUserdata "list(object({path=string, content=string}))" {
    default = null
    description = "A complex object"
}

resource main "aws:ec2:Vpc" {
	cidrBlock = "10.100.0.0/16"
	tags = {
		"Name": vpcTag
	}
}