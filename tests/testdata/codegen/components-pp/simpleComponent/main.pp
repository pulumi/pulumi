resource firstPassword "random:index/randomPassword:RandomPassword" {
  length = 16
  special = true
}

resource secondPassword "random:index/randomPassword:RandomPassword" {
  length = 16
  special = true
}