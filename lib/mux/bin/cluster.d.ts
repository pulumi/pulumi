import * as mu from "mu";
export default class Cluster extends mu.Stack {
    constructor(args: ClusterArgs);
    private createAWSCloudResources(args);
}
export interface ClusterArgs {
    name: string;
    arch: Arch;
}
export interface Arch {
    cloud: Cloud;
    scheduler?: Scheduler;
}
export declare type Cloud = "aws" | "gcp" | "azure" | "vmware";
export declare type Scheduler = "swarm" | "kubernetes" | "mesos" | "awsecs" | "gcpgke" | "azurecs";
