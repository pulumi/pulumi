import pulumi
import pulumi_selfref as selfref

root = selfref.Node("root")
child = selfref.Node("child",
    parent=root,
    parents=[root],
    named_parents={
        "root": root,
    },
    parent_or_name=root)
