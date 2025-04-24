output strVar {
	__logicalName = "strVar"
	value = "foo"
}

output arrVar {
	__logicalName = "arrVar"
	value = [
		"fizz",
		"buzz"
	]
}

output readme {
	__logicalName = "readme"
	value = readFile("./Pulumi.README.md")
}
