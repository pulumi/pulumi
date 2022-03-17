// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Provider implements pulumi.provider.Provider {
    public readonly version = "0.0.1";
    constructor(public readonly schema?: string) {
    }
}

export function main(args: string[]) {
    const schema = process.env.INCLUDE_SCHEMA ? `{"hello": "world"}` : undefined;
    return pulumi.provider.main(new Provider(schema), args);
}

main(process.argv.slice(2));
