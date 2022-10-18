resource rawkode "eks:index:Cluster" {
	__logicalName = "rawkode"
	instanceType = "t2.medium"
	desiredCapacity = 2
	minSize = 1
	maxSize = 2
}

resource stack72 "eks:index:Cluster" {
	__logicalName = "stack72"
	instanceType = "t2.medium"
	desiredCapacity = 4
	minSize = 1
	maxSize = 8
}
