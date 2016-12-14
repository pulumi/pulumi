// Copyright 2016 Marapongo, Inc. All rights reserved.

module mu
import mu/clouds/aws

// A base Mu cluster that can run on any cloud/scheduler target.
service Cluster {
    ctor() {
        switch context.arch.cloud {
        case "aws":
            new aws.Cluster {}
        default:
            panic("Unrecognized cloud target: %v", context.arch.cloud)
        }
    }
}

