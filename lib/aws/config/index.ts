// Copyright 2016 Pulumi, Inc. All rights reserved.

// region configures the target region for a deployment.  The provider explicitly does not recognize AWS_REGION,
// to minimize the possibility of accidental deployment differences due to a changing environment variable.
export let region:
    "us-east-1"      | // US East (N. Virginia)
    "us-east-2"      | // US East (Ohio)
    "us-west-1"      | // US West (N. California)
    "us-west-2"      | // US West (Oregon)
    "ca-central"     | // Canada (Central)
    "ap-south-1"     | // Asia Pacific (Mumbai)
    "ap-northeast-1" | // Asia Pacific (Tokyo)
    "ap-northeast-2" | // Asia Pacific (Seoul)
    "ap-southeast-1" | // Asia Pacific (Singapore)
    "ap-southeast-2" | // Asia Pacific (Sydney)
    "eu-central-1"   | // EU (Frankfurt)
    "eu-west-1"      | // EU (Ireland)
    "eu-west-2"      | // EU (London)
    "sa-east-1"      | // South America (Sao Paulo)
    undefined;

// accessKeyId configures the access key ID used to access AWS.  This is a secret.  If not provided, the
// provider will look in the standard places (~/.aws/credentials, AWS_ACCESS_KEY_ID, etc).
export let accessKeyId: string | undefined;

// secretAcessKey configures the secret access key used to access AWS.  This is a secret.  If not provided, the
// provider will look in the standard places (~/.aws/credentials, AWS_SECRET_ACCESS_KEY, etc).
export let secretAccessKey: string | undefined; // the secret access key used to access AWS.

