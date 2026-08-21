# A resource whose `elementType` property collides with the `ElementType()` method that
# generated Go SDK types must implement.
resource "elem" "reservednames:index:ElementType" {
    elementType = {
        elementType = "nested"
    }
}

# The whole object checks that languages serialize typed objects in stack outputs with
# their wire-format (camelCase) keys; the nested string exercises traversal through the
# renamed members.
output "elementType" {
    value = elem.elementType
}

output "nested" {
    value = elem.elementType.elementType
}
