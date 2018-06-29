import * as pulumi from "@pulumi/pulumi";
import * as dynamic from "@pulumi/pulumi/dynamic";
export declare class Provider implements dynamic.ResourceProvider {
    static readonly instance: Provider;
    private id;
    check(olds: any, news: any): Promise<dynamic.CheckResult>;
    create(inputs: any): Promise<dynamic.CreateResult>;
    update(id: pulumi.ID, olds: any, news: any): Promise<dynamic.UpdateResult>;
}
export declare class Resource extends dynamic.Resource {
    readonly state: pulumi.Output<number>;
    constructor(name: string, num: pulumi.Input<number>, opts?: pulumi.ResourceOptions);
}
