import * as pulumi from '@pulumi/pulumi';
import { v4 as uuidv4 } from "uuid";

const provider = {
    async create() {
        throw new Error("Oops");
    }
};
class MyResource extends pulumi.dynamic.Resource {
    constructor(name) {
        super(provider, name, {}, {});
    }
}

async function program() {
    const res = new MyResource("res")
    // There's a create error in `res`, so this will never resolve. Pulumi
    // should know about the failed resource creation and fail the deployment
    // (and thus exit).
    return { res };
}

async function main() {
    const stack = await pulumi.automation.LocalWorkspace.createOrSelectStack({
        stackName: 'test' + uuidv4(),
        projectName: 'resource-error-in-auto-api',
        program,
    });
    return stack.up({})
}

main().catch(err => {
    console.error(err)
    process.exit(1);
});
