import * as pulumi from "@pulumi/pulumi";

export class MyRandom extends pulumi.ComponentResource {
  public readonly randomID: pulumi.Output<string>;

  constructor(name: string, opts: pulumi.ResourceOptions) {
    super("pkg:index:MyRandom", name, {}, opts);
    this.randomID = pulumi.output(`${name}-${Math.floor(Math.random() * 1000)}`);
    this.registerOutputs({ randomID: this.randomID });
  }
}
