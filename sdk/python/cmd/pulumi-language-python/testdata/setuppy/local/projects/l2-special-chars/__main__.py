import pulumi
import pulumi_specialchars as specialchars

# The nested object type has property names containing characters that are not legal
# identifier characters in most languages: "@timestamp" and "entity.name".
first = specialchars.Thing("first", value="hello")
pulumi.export("itemOutput", first.item)
pulumi.export("invoked", specialchars.get_item_output(value="world"))
