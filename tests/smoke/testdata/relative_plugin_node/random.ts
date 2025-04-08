// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

export class Random extends pulumi.Resource {
    result!: pulumi.Output<string | undefined>;
  
    constructor(name: string, length: number, opts?: pulumi.ResourceOptions) {
      const inputs: any = {};
      inputs["length"] = length;
      super("testprovider:index:Random", name, true, inputs, opts);
    }
  }