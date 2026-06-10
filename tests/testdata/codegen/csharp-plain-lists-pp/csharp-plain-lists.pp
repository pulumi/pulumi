resource network "plain-lists:index:Network" {
    subnetSpecs = [
        { type = "Public", cidrMask = 22 },
        { type = "Private", cidrMask = 20 }
    ]
}
