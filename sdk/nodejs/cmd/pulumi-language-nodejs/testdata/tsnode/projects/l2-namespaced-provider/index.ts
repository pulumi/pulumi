import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";
import * as namespaced from "@a-namespace/namespaced";

const componentRes = new component.ComponentCustomRefOutput("componentRes", {value: "foo-bar-baz"});
const res = new namespaced.Resource("res", {
    value: true,
    resourceRef: componentRes.ref,
});
