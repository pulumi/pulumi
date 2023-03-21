config input string {
    description = "A simple input"
}

config cidrBlocks "map(string)" {
    description = "The main CIDR blocks for the VPC"
}

config ipAddress "list(int)" { }

resource password "random:index/randomPassword:RandomPassword" {
  length = 16
  special = true
  overrideSpecial = input
}

component simpleComponent "../simpleComponent" {}

output result {
    value = password.result
}