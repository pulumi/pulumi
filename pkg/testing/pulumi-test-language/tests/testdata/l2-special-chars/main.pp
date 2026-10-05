// The nested object type has property names containing characters that are not legal
// identifier characters in most languages: "@timestamp" and "entity.name".
resource "first" "specialchars:index:Thing" {
    value = "hello"
}

// Whole objects in stack outputs keep their wire-format keys
output "itemOutput" {
    value = first.item
}

output "invoked" {
    value = invoke("specialchars:index:getItem", {
        value = "world"
    })
}
