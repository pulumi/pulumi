// Copyright 2024 Pulumi Corporation.

import * as pulumi from "@pulumi/pulumi";

class SimpleProvider implements pulumi.provider.Provider {
    readonly version: string = "2.0.0"

    constructor() {}

    async getSchema(): Promise<string> {
      const properties = {
        "value": {
          "type": "boolean",
        },
      }

      const required = ["value"]

      return JSON.stringify({
        name: "simple",
        version: "2.0.0",
        resources: {
          "simple:index:Resource": {
            type: "object",
            properties,
            required,
            inputProperties: properties,
            requiredInputs: required,
          },
        },
      })
    }

  async check(urn: pulumi.URN, olds: any, news: any): Promise<pulumi.provider.CheckResult> {
    return {
      inputs: news,
    }
  }

  async create(urn: pulumi.URN, inputs: any): Promise<pulumi.provider.CreateResult> {
    return {
      id: "id",
      outs: inputs,
    }
  }
}

const p = new SimpleProvider();
pulumi.provider.main(p, process.argv.slice(2));
