import * as pulumi from "@pulumi/pulumi";

export = async () => {
    // Keywords in various languages should be renamed and work.
    const _class = "class_output_string";
    const _export = "export_output_string";
    const _import = "import_output_string";
    const mod = "mod_output_string";
    const object = {
        object: "object_output_string",
    };
    const self = "self_output_string";
    const _this = "this_output_string";
    const _if = "if_output_string";
    return {
        "class": _class,
        "export": _export,
        "import": _import,
        mod: mod,
        object: object,
        self: self,
        "this": _this,
        "if": _if,
    };
}
