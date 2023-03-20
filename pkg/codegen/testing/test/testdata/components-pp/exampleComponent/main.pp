config input string { }

config cidrBlocks "map(string)" { }

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