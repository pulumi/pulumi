# A resource whose `elementType` property collides with the `ElementType()` method that
# generated Go SDK types must implement.
resource "elem" "reservednames:index:ElementType" { }

output "elementType" {
    value = elem.elementType.elementType
}
