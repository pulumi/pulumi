resource "custom1" "component-property-deps:index:Custom" {
    value = "hello"
}

resource "custom2" "component-property-deps:index:Custom" {
    value = "world"
}

resource "component1" "component-property-deps:index:Component" {
    resource = custom1
    resourceList = [custom1, custom2]
    resourceMap = {
        "one" = custom1,
        "two" = custom2,
    }
}

output "propertyDepsFromCall" {
    value = call(component1, "refs", {
        resource = custom1
        resourceList = [custom1, custom2]
        resourceMap = {
            "one" = custom1,
            "two" = custom2,
        }
    }).result
}
