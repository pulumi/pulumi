// Copyright 2016 Marapongo, Inc. All rights reserved.

module mu
import mu/clouds/aws

// A base Mu cluster that can run on any cloud/scheduler target.
service Cluster {
    new() {
        switch context.arch.cloud {
        case "aws":
            cf := new aws.Cluster {}
        default:
            mu.Panic("Unrecognized cloud target: %v", context.arch.cloud)
        }
    }
}

