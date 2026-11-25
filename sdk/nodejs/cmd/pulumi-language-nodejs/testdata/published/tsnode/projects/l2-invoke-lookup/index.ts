import * as pulumi from "@pulumi/pulumi";
import * as lookup_invoke from "@pulumi/lookup-invoke";

export = async () => {
    const widget = new lookup_invoke.Widget("widget", {name: "resource"});
    // getWidget is a root-module function whose name clashes with the Widget
    // resource's getter in some SDKs (the Go SDK exports it as LookupWidget).
    const fromLiteral = lookup_invoke.getWidgetOutput({
        name: "literal",
    });
    const fromResource = lookup_invoke.getWidgetOutput({
        name: widget.name,
    });
    return {
        fromLiteral: fromLiteral.result,
        fromResource: fromResource.result,
    };
}
