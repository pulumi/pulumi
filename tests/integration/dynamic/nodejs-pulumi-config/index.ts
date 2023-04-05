// Copyright 2016-2023, Pulumi Corporation.

import * as pulumi from '@pulumi/pulumi'

class CustomResource extends pulumi.dynamic.Resource {
  constructor (name: string, opts?: pulumi.ResourceOptions) {
    super(new DummyResourceProvider(), name, {}, opts, "custom-provider", "CustomResource")
  }
}

class DummyResourceProvider implements pulumi.dynamic.ResourceProvider {
  async create (props: any): Promise<pulumi.dynamic.CreateResult> {
    var config = JSON.parse(process.env.PULUMI_CONFIG)

    return { id: config["id"], outs: {} }
  }
}

const resource = new CustomResource('resource-name')

export const rid = resource.id
