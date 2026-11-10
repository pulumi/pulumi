import pulumi
import pulumi_objectref as objectref

widget = objectref.referencingfirst.Widget("widget", settings={
    "name": "widget",
})
gadget = objectref.referencingfirst.Gadget("gadget", widget_ref={
    "widget": widget,
})
alpha = objectref.referencedfirst.Alpha("alpha", settings={
    "name": "alpha",
})
beta = objectref.referencedfirst.Beta("beta", alpha_ref={
    "alpha": alpha,
})
