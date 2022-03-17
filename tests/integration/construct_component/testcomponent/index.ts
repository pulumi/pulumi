import * as pulumi from "@pulumi/pulumi";
import * as dynamic from "@pulumi/pulumi/dynamic";
import * as provider from "@pulumi/pulumi/provider";

let currentID = 0;

class Resource extends dynamic.Resource {
    constructor(name: string, echo: pulumi.Input<any>, opts?: pulumi.CustomResourceOptions) {
        const provider = {
            create: async (inputs: any) => ({
                id: (currentID++).toString(),
                outs: undefined,
            }),
        };

        super(provider, name, {echo}, opts);
    }
}

class Component extends pulumi.ComponentResource {
    public readonly echo: pulumi.Output<any>;
    public readonly childId: pulumi.Output<pulumi.ID>;
    public readonly secret: pulumi.Output<string>;

    constructor(name: string, echo: pulumi.Input<any>, secret: pulumi.Output<string>, opts?: pulumi.ComponentResourceOptions) {
        super("testcomponent:index:Component", name, {}, opts);

        this.echo = pulumi.output(echo);
        this.childId = (new Resource(`child-${name}`, echo, {parent: this})).id;
        this.secret = secret;

        this.registerOutputs({
            echo: this.echo,
            childId: this.childId,
            secret: this.secret,
        })
    }
}

class Provider implements provider.Provider {
    public readonly version = "0.0.1";

    construct(name: string, type: string, inputs: pulumi.Inputs,
              options: pulumi.ComponentResourceOptions): Promise<provider.ConstructResult> {
        if (type != "testcomponent:index:Component") {
            throw new Error(`unknown resource type ${type}`);
        }

        const config = new pulumi.Config();
        const secretKey = "secret";
        const fullSecretKey = `${config.name}:${secretKey}`;
        // use internal pulumi prop to check secretness
        const isSecret = (pulumi.runtime as any).isConfigSecret(fullSecretKey); 
        if (!isSecret) {
            throw new Error(`expected config with key "${secretKey}" to be secret.`)
        }
        const secret = config.requireSecret(secretKey);


        const component = new Component(name, inputs["echo"], secret, options);
        return Promise.resolve({
            urn: component.urn,
            state: {
                echo: component.echo,
                childId: component.childId,
                secret: secret,
            },
        });
    }
}

export function main(args: string[]) {
    return provider.main(new Provider(), args);
}

main(process.argv.slice(2));
