import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

interface SubmoduleArgs {
    submoduleListVar: pulumi.Input<string[]>,
    submoduleFilterCond: pulumi.Input<boolean>,
    submoduleFilterVariable: pulumi.Input<number>,
}

export class Submodule extends pulumi.ComponentResource {
    constructor(name: string, args: SubmoduleArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:Submodule", name, args, opts);
        const submoduleRes: simple.Resource[] = [];
pulumi.output(args.submoduleListVar).apply(__convert => __convert.map((v, k) => [k, v]).filter(([k, v]) => pulumi.output(args.submoduleFilterCond)).reduce((__obj, [k, v]) => ({ ...__obj, [k]: v }))).apply(rangeBody => {
            for (const range of Object.entries(rangeBody).map(([k, v]) => ({key: k, value: v}))) {
                submoduleRes.push(new simple.Resource(`${name}-submoduleRes-${range.key}`, {value: true}, {
                parent: this,
            }));
            }
        });

        const submoduleResWithApplyFilter: simple.Resource[] = [];
pulumi.all([pulumi.output(args.submoduleListVar), pulumi.output(args.submoduleFilterVariable)]).apply(([__convert, __convert1]) => __convert.map((v, k) => [k, v]).filter(([k, v]) => __convert1 == 1).reduce((__obj, [k, v]) => ({ ...__obj, [k]: v }))).apply(rangeBody => {
            for (const range of Object.entries(rangeBody).map(([k, v]) => ({key: k, value: v}))) {
                submoduleResWithApplyFilter.push(new simple.Resource(`${name}-submoduleResWithApplyFilter-${range.key}`, {value: true}, {
                parent: this,
            }));
            }
        });

        this.registerOutputs();
    }
}
