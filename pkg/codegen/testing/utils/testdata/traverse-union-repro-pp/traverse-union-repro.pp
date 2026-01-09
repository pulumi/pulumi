resource "test" "aws:fsx:OpenZfsFileSystem" {
  storageCapacity    = 64
  subnetIds          = [aws_subnet.test1.id]
  deploymentType     = "SINGLE_AZ_1"
  throughputCapacity = 64
}
