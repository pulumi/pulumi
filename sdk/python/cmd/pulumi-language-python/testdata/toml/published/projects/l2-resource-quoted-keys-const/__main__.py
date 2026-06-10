import pulumi
import pulumi_manifest as manifest

first = manifest.Resource("first",
    kind="Manifest",
    metadata={
        "name": "first",
        "labels": {
            "app": "first",
        },
    },
    spec={
        "replicas": 1,
        "template": {
            "metadata": {
                "name": "inner",
            },
            "containers": [{
                "name": "app",
                "image": "nginx",
                "ports": [80],
            }],
        },
    })
pulumi.export("kind", first.kind)
