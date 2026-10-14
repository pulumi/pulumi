import pulumi
import pulumi_myborrow as myborrow

widget = myborrow.Widget("widget")
pulumi.export("metadataName", widget.metadata.name)
