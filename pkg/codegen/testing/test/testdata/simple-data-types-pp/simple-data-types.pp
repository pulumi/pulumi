basicStrVar = "foo"

output strVar {
	__logicalName = "strVar"
	value = basicStrVar
}

output computedStrVar {
	__logicalName = "computedStrVar"
	value = "${basicStrVar}/computed"
}

output strArrVar {
	__logicalName = "strArrVar"
	value = [
		"fiz",
		"buss"
	]
}

output intVar {
	__logicalName = "intVar"
	value = 42
}

output intArr {
	__logicalName = "intArr"
	value = [
		1,
		2,
		3,
		4,
		5
	]
}

output readme {
	__logicalName = "readme"
	value = readFile("./Pulumi.README.md")
}
