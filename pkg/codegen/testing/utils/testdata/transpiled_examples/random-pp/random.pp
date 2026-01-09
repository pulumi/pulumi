resource randomPassword "random:index/randomPassword:RandomPassword" {
	__logicalName = "randomPassword"
	length = 16
	special = true
	overrideSpecial = "_%@"
}

output password {
	__logicalName = "password"
	value = randomPassword.result
}
