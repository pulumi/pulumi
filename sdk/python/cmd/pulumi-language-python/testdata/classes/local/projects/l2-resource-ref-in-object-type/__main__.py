import pulumi
import pulumi_objectref as objectref

widget = objectref.referencingfirst.Widget("widget", settings=objectref.referencingfirst.WidgetSettingsArgs(
    name="widget",
))
gadget = objectref.referencingfirst.Gadget("gadget", widget_ref=objectref.referencingfirst.WidgetRefArgs(
    widget=widget,
))
alpha = objectref.referencedfirst.Alpha("alpha", settings=objectref.referencedfirst.AlphaSettingsArgs(
    name="alpha",
))
beta = objectref.referencedfirst.Beta("beta", alpha_ref=objectref.referencedfirst.AlphaRefArgs(
    alpha=alpha,
))
