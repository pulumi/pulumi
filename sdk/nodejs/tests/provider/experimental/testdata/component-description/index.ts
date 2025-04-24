// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

/**
 * myClassType comment
 */
export class MyClassType {
    /**
     * aString comment
     */
    aString: string;
}

/**
 * myInterfaceType comment
 */
export interface MyInterfaceType {
    /**
     * aNumber comment
     */
    aNumber: number;
}

export interface MyComponentArgs {
    /**
     * anInterfaceType doc comment
     */
    anInterfaceType: MyInterfaceType;
    /**
     * aClassType comment
     */
    aClassType: MyClassType;
    /**
     * inputMap comment
     */
    inputMapOfInterfaceTypes: pulumi.Input<{ [key: string]: MyInterfaceType }>;
    /**
     * anArchive comment
     */
    anArchive: pulumi.asset.Archive;
    /**
     * anAsset comment
     */
    anAsset: pulumi.asset.Asset;
    /**
     * anArray comment
     */
    anArray: string[];
}

/**
 * This is a description of MyComponent
 * It can span multiple lines
 */
export class MyComponent extends pulumi.ComponentResource {
    /**
     * out_string_map comment
     */
    outStringMap: pulumi.Output<{ [key: string]: number }>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
    }
}
