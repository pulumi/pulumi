import pulumi
import pulumi_reservednames as reservednames

# A resource whose `elementType` property collides with the `ElementType()` method that
# generated Go SDK types must implement.
elem = reservednames.ElementType("elem")
pulumi.export("elementType", elem.element_type.element_type)
