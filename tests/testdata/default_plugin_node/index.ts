// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export interface RandomStringArgs {
    length: pulumi.Input<number>;
}

// Register a random resource manually so there's no version set
export class RandomString extends pulumi.CustomResource {
    constructor(name: string, args: RandomStringArgs, opts?: pulumi.ResourceOptions) {
      super("random:index/randomString:RandomString", name, args, opts);
    }
  }

new RandomString("random", { length: 10 });
