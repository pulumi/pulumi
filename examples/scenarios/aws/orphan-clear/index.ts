// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as aws from "@lumi/aws";
import * as lumirt from "@lumi/lumirt";
let results = aws.ec2.SecurityGroup.query();
for (let i = 0; i < (<any>results).length; i++) {
    let sg = results[i];
    lumirt.printf(sg.groupName);
    lumirt.printf("\n");
}

// For now, we simply print out the names.  Eventually, this code should delete orphaned groups.
//
//     TODO[pulumi/lumi#295]: we need support for filtering so we can query for only orphaned groups.
//     TODO[pulumi/lumi#296]: we need support for imperative deletions since deletes aren't diff-based.
//
// After support for those two things, we should end up with something like:
//
//     for (let sg of aws.ec2.SecurityGroup.query()) {
//         if (aws.ec2.Instance.query({ securityGroups: sg.name }).length === 0) {
//             sg.delete();
//         }
//     }
