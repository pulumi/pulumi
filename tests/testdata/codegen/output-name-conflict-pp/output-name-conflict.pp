config cidrBlock string {
    default = "Test config variable"
}

output cidrBlock {
  value = cidrBlock
}