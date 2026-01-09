resource vpc "awsx:ec2:Vpc" {
    subnetSpecs = [
        { type = "Public", cidrMask = 22 },
        { type = "Private", cidrMask = 20 }
    ]
}