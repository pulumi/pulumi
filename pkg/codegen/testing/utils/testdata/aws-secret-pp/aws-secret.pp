resource dbCluster "aws:rds:Cluster" {
	masterPassword = secret("foobar")
}
