import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

export = async () => {
    const _class = new simple.Resource("class", {value: true});
    const _export = new simple.Resource("export", {value: true});
    const mod = new simple.Resource("mod", {value: true});
    const _import = new simple.Resource("import", {value: true});
    // TODO(pulumi/pulumi#18246): Pcl should support scoping based on resource type just like HCL does in TF so we can uncomment this.
    // output "import" {
    //   value = Resource["import"]
    // }
    const object = new simple.Resource("object", {value: true});
    const self = new simple.Resource("self", {value: true});
    const _this = new simple.Resource("this", {value: true});
    return {
        "class": _class,
        "export": _export,
        mod: mod,
        object: object,
        self: self,
        "this": _this,
    };
}
