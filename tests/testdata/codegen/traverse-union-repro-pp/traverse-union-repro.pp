resource "test" "infra:index:FileSystem" {
  storageCapacity    = 64
  subnetIds          = [aws_subnet.test1.id]
  deploymentType     = "SINGLE_AZ_1"
  throughputCapacity = 64
}
