import * as as mu from 'mu';

// A special service that simply emits a CloudFormation template.
// @name: aws/x/cf
export default class CloudFormation extends mu.Extension {
    constructor(ctx: mu.Context, args: CloudFormationArgs) {
        super(ctx);
        // TODO: encode the special translation logic as code (maybe as an overridden method).
    }
}

export interface CloudFormationArgs {
    // The CF resource name.
    readonly resource: string;
    // An optional list of properties to map.
    readonly properties?: Map<string, string>;
    // An optional set of arbitrary key/values to merge with the mapped ones.
    readonly extraProperties?: Map<string, any>;
    // An optional list of other CloudFormation resources that this depends on.
    readonly dependsOn?: mu.Stack[];
}

