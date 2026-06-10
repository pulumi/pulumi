import pulumi
import pulumi_manifest as manifest

first = manifest.Resource("first",
    kind="Manifest",
    metadata=manifest.MetadataArgs(
        name="first",
        labels={
            "app": "first",
        },
    ),
    spec=manifest.SpecArgs(
        replicas=1,
        template=manifest.TemplateArgs(
            metadata=manifest.MetadataArgs(
                name="inner",
            ),
            containers=[manifest.ContainerArgs(
                name="app",
                image="nginx",
                ports=[80],
            )],
        ),
    ))
pulumi.export("kind", first.kind)
