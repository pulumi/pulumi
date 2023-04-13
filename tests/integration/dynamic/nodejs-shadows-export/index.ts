// Copyright 2016-2023, Pulumi Corporation.

import { Input, Output,  CustomResourceOptions, output } from "@pulumi/pulumi"
import { ResourceProvider, Resource } from "@pulumi/pulumi/dynamic";

interface ReproResourceInputs {
    example: Input<string>
}

interface ReproInputs {
    example: string
}

class ReproProvider implements ResourceProvider {
    async create(inputs: ReproInputs) {
        return { id: "nothing", outs: {
            example: output(inputs.example)
        }};
    }
}

class Repro extends Resource {
    constructor(name: string, args: ReproResourceInputs, opts?: CustomResourceOptions) {
        super(new ReproProvider(), name, args, opts);
    }
}

const reproResource = new Repro("test", { example: "words"});