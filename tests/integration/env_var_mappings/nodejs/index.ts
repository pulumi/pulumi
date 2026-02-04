// Copyright 2016-2026, Pulumi Corporation.  All rights reserved.

import { TestProvider } from "./provider";

// Create a provider with envVarMappings to remap environment variables.
// When the provider process starts, if MY_VAR is set, the provider will
// see PROVIDER_VAR with MY_VAR's value instead.
const prov = new TestProvider("prov", {
    envVarMappings: {
        "MY_VAR": "PROVIDER_VAR",
        "OTHER_VAR": "TARGET_VAR",
    },
});
