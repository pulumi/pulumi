// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This file defines some maps that correspond to the recommended AWS Marketplace values:
//     http://docs.aws.amazon.com/servicecatalog/latest/adminguide/catalogs_marketplace-products.html
// Ultimately, this lets us choose a recommended Amazon Linux AMI; see:
//     https://aws.amazon.com/amazon-linux-ami/instance-type-matrix/

import * as config from "../config";

// instanceTypeArch is a map of instance type to its architecture.
export let instanceTypeArch: {
    [instanceType: string]: string,
} = {
    "t1.micro"   : "PV64" ,
    "t2.nano"    : "HVM64",
    "t2.micro"   : "HVM64",
    "t2.small"   : "HVM64",
    "t2.medium"  : "HVM64",
    "t2.large"   : "HVM64",
    "m1.small"   : "PV64" ,
    "m1.medium"  : "PV64" ,
    "m1.large"   : "PV64" ,
    "m1.xlarge"  : "PV64" ,
    "m2.xlarge"  : "PV64" ,
    "m2.2xlarge" : "PV64" ,
    "m2.4xlarge" : "PV64" ,
    "m3.medium"  : "HVM64",
    "m3.large"   : "HVM64",
    "m3.xlarge"  : "HVM64",
    "m3.2xlarge" : "HVM64",
    "m4.large"   : "HVM64",
    "m4.xlarge"  : "HVM64",
    "m4.2xlarge" : "HVM64",
    "m4.4xlarge" : "HVM64",
    "m4.10xlarge": "HVM64",
    "c1.medium"  : "PV64" ,
    "c1.xlarge"  : "PV64" ,
    "c3.large"   : "HVM64",
    "c3.xlarge"  : "HVM64",
    "c3.2xlarge" : "HVM64",
    "c3.4xlarge" : "HVM64",
    "c3.8xlarge" : "HVM64",
    "c4.large"   : "HVM64",
    "c4.xlarge"  : "HVM64",
    "c4.2xlarge" : "HVM64",
    "c4.4xlarge" : "HVM64",
    "c4.8xlarge" : "HVM64",
    "g2.2xlarge" : "HVMG2",
    "g2.8xlarge" : "HVMG2",
    "r3.large"   : "HVM64",
    "r3.xlarge"  : "HVM64",
    "r3.2xlarge" : "HVM64",
    "r3.4xlarge" : "HVM64",
    "r3.8xlarge" : "HVM64",
    "i2.xlarge"  : "HVM64",
    "i2.2xlarge" : "HVM64",
    "i2.4xlarge" : "HVM64",
    "i2.8xlarge" : "HVM64",
    "d2.xlarge"  : "HVM64",
    "d2.2xlarge" : "HVM64",
    "d2.4xlarge" : "HVM64",
    "d2.8xlarge" : "HVM64",
    "hi1.4xlarge": "HVM64",
    "hs1.8xlarge": "HVM64",
    "cr1.8xlarge": "HVM64",
    "cc2.8xlarge": "HVM64",
};

// regionArchLinuxAMI is a map from region to inner maps from architecture to the recommended Linux AMI.
export let regionArchLinuxAMI: {
    [region: string]: { [arch: string]: string; },
} = {
    "us-east-1": {
        "PV64" : "ami-2a69aa47",
        "HVM64": "ami-6869aa05",
        "HVMG2": "ami-648d9973",
    },
    "us-west-2": {
        "PV64" : "ami-7f77b31f",
        "HVM64": "ami-7172b611",
        "HVMG2": "ami-09cd7a69",
    },
    "us-west-1": {
        "PV64" : "ami-a2490dc2",
        "HVM64": "ami-31490d51",
        "HVMG2": "ami-1e5f0e7e",
    },
    "eu-west-1": {
        "PV64" : "ami-4cdd453f",
        "HVM64": "ami-f9dd458a",
        "HVMG2": "ami-b4694ac7",
    },
    "eu-west-2": {
        "PV64" : "NOT_SUPPORTED",
        "HVM64": "ami-886369ec",
        "HVMG2": "NOT_SUPPORTED",
    },
    "eu-central-1": {
        "PV64" : "ami-6527cf0a",
        "HVM64": "ami-ea26ce85",
        "HVMG2": "ami-de5191b1",
    },
    "ap-northeast-1": {
        "PV64" : "ami-3e42b65f",
        "HVM64": "ami-374db956",
        "HVMG2": "ami-df9ff4b8",
    },
    "ap-northeast-2": {
        "PV64" : "NOT_SUPPORTED",
        "HVM64": "ami-2b408b45",
        "HVMG2": "NOT_SUPPORTED",
    },
    "ap-southeast-1": {
        "PV64" : "ami-df9e4cbc",
        "HVM64": "ami-a59b49c6",
        "HVMG2": "ami-8d8d23ee",
    },
    "ap-southeast-2": {
        "PV64" : "ami-63351d00",
        "HVM64": "ami-dc361ebf",
        "HVMG2": "ami-cbaf94a8",
    },
    "ap-south-1": {
        "PV64" : "NOT_SUPPORTED",
        "HVM64": "ami-ffbdd790",
        "HVMG2": "ami-decdbab1",
    },
    "us-east-2": {
        "PV64" : "NOT_SUPPORTED",
        "HVM64": "ami-f6035893",
        "HVMG2": "NOT_SUPPORTED",
    },
    "ca-central-1": {
        "PV64" : "NOT_SUPPORTED",
        "HVM64": "ami-730ebd17",
        "HVMG2": "NOT_SUPPORTED",
    },
    "sa-east-1": {
        "PV64" : "ami-1ad34676",
        "HVM64": "ami-6dd04501",
        "HVMG2": "NOT_SUPPORTED",
    },
    "cn-north-1": {
        "PV64" : "ami-77559f1a",
        "HVM64": "ami-8e6aa0e3",
        "HVMG2": "NOT_SUPPORTED",
    },
};

// getLinuxAMI gets the recommended Linux AMI for the given instance in the current AWS region.
export function getLinuxAMI(instanceType: string): string {
    let region = config.requireRegion();
    let arch = instanceTypeArch[instanceType];
    return regionArchLinuxAMI[region][arch];
}

