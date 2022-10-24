import * as pulumi from "@pulumi/pulumi";
import * as docker from "@pulumi/docker";

const latest = docker.getRemoteImage({
    name: "nginx",
});
const ubuntu = new docker.RemoteImage("ubuntu", {name: "ubuntu:precise"});
export const remoteImageId = latest.then(latest => latest.id);
export const ubuntuImage = ubuntu.name;
