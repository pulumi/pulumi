import pulumi
import pulumi_lookup_invoke as lookup_invoke

widget = lookup_invoke.Widget("widget", name="resource")
# getWidget is a root-module function whose name clashes with the Widget
# resource's getter in some SDKs (the Go SDK exports it as LookupWidget).
from_literal = lookup_invoke.get_widget_output(name="literal")
from_resource = lookup_invoke.get_widget_output(name=widget.name)
pulumi.export("fromLiteral", from_literal.result)
pulumi.export("fromResource", from_resource.result)
