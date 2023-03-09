config input string {
}

resource password "random:index/randomPassword:RandomPassword" {
  length = 16
  special = true
  overrideSpecial = input
}

component simpleComponent "../simpleComponent" {}

output result {
    value = password.result
}