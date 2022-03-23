import * as pulumi from "@pulumi/pulumi";

class Random extends pulumi.Resource {
  result!: pulumi.Output<string | undefined>;

  constructor(name: string, length: number, opts?: pulumi.ResourceOptions) {
    const inputs: any = {};
    inputs["length"] = length;
    super("testprovider:index:Random", name, true, inputs, opts);
  }
}

class RandomProvider extends pulumi.ProviderResource {
  constructor(name: string, opts?: pulumi.ResourceOptions) {
    super("testprovider", name, {}, opts);
  }
}

const r = new Random("default", 10, {
  pluginDownloadURL: "get.com",
});
export const defaultProvider = r.result;

const provider = new RandomProvider("explicit", {
  pluginDownloadURL: "get.pulumi/test/providers",
});

new Random("explicit", 8, { provider: provider });
