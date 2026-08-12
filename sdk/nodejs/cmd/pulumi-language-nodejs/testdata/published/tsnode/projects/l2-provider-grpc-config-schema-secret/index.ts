import * as pulumi from "@pulumi/pulumi";
import * as config_grpc from "@pulumi/config-grpc";

// This provider covers scenarios where configuration properties are marked as secret in the schema.
const config_grpc_provider = new config_grpc.Provider("config_grpc_provider", {
    secretString1: "SECRET",
    secretInt1: 16,
    secretNum1: 123456.789,
    secretBool1: true,
    listSecretString1: [
        "SECRET",
        "SECRET2",
    ],
    mapSecretString1: {
        key1: "SECRET",
        key2: "SECRET2",
    },
    objSecretString1: {
        secretX: "SECRET",
    },
});
const config = new config_grpc.ConfigFetcher("config", {}, {
    provider: config_grpc_provider,
});
