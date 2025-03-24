// This file has syntax errors but shouldn't matter since it's not imported
export class BrokenComponent extends ComponentResource {
    // Missing import, syntax error
    constructor(name: string, args: BrokenArgs) {
        super("provider:broken:Component", name);
        this.missing = args.doesntExist; // More errors
    }
}
