import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/pulumi/provider";

interface ComponentArgs {
    message: pulumi.Input<string>;
    nested: pulumi.Input<{
        value: pulumi.Input<string>;
    }>;
}

class Component extends pulumi.ComponentResource {
    constructor(name: string, args: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, {}, opts);

        // These `apply`s should not run.
        pulumi.output(args.message).apply(v => { console.log("should not run (message)"); process.exit(1); });
        pulumi.output(args.nested).apply(v => { console.log("should not run (nested)"); process.exit(1); });
    }
}

class Provider implements provider.Provider {
    public readonly version = "0.0.1";

    construct(name: string, type: string, inputs: pulumi.Inputs,
              options: pulumi.ComponentResourceOptions): Promise<provider.ConstructResult> {
        if (type != "testcomponent:index:Component") {
            throw new Error(`unknown resource type ${type}`);
        }

        const component = new Component(name, <ComponentArgs>inputs, options);
        return Promise.resolve({
            urn: component.urn,
            state: {},
        });
    }
}

export function main(args: string[]) {
    return provider.main(new Provider(), args);
}

main(process.argv.slice(2));
