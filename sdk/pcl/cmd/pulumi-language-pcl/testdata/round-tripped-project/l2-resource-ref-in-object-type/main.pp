resource "widget" "objectref:referencingFirst:Widget" {
    settings = { name = "widget" }
}

resource "gadget" "objectref:referencingFirst:Gadget" {
    widgetRef = { widget = widget }
}

resource "alpha" "objectref:referencedFirst:Alpha" {
    settings = { name = "alpha" }
}

resource "beta" "objectref:referencedFirst:Beta" {
    alphaRef = { alpha = alpha }
}
