import * as pulumi from "@pulumi/pulumi";
import * as infra from "@pulumi/infra";

const test = new infra.FileSystem("test", {
    storageCapacity: 64,
    subnetIds: [aws_subnet.test1.id],
    deploymentType: "SINGLE_AZ_1",
    throughputCapacity: 64,
});
