import pulumi
import pulumi_nestedobject as nestedobject

receiver_ignore = nestedobject.Receiver("receiverIgnore", details=[nestedobject.DetailArgs(
    key="a",
    value="b",
)],
opts = pulumi.ResourceOptions(ignore_changes=["details[0].key"]))
map_ignore = nestedobject.MapContainer("mapIgnore", tags={
    "env": "prod",
},
opts = pulumi.ResourceOptions(ignore_changes=["tags.env"]))
no_ignore = nestedobject.Target("noIgnore", name="nothing")
