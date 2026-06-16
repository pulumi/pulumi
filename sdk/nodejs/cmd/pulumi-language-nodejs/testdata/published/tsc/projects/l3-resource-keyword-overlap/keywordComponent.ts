import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

interface KeywordComponentArgs {
    /**
     * An input passed to the component
     */
    input: pulumi.Input<boolean>,
}

export class KeywordComponent extends pulumi.ComponentResource {
    public result: pulumi.Output<boolean>;
    constructor(name: string, args: KeywordComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:KeywordComponent", name, args, opts);
        // A resource named `this` collides with the receiver pointer of the
        // ComponentResource class generated for this component. NodeJS must rename the
        // resource variable (e.g. to `_this`) while keeping the `parent: this` pointer
        // intact.
        const _this = new simple.Resource(`${name}-this`, {value: args.input}, {
            parent: this,
        });

        // Referencing `this` exercises that the rename is applied to references too, not
        // just the declaration. The name `parent` also overlaps with the `parent`
        // resource-option key, which must not be confused with this resource variable.
        const parent = new simple.Resource(`${name}-parent`, {value: _this.value}, {
            parent: this,
        });

        this.result = parent.value;
        this.registerOutputs({
            result: parent.value,
        });
    }
}
