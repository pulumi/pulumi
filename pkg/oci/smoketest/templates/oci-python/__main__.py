# A minimal OCI-mode Pulumi program. It runs as a container (runtime: oci), built from
# the Dockerfile in this directory. Add resources as usual; run `pulumi package add
# <provider>` to generate and wire a provider SDK.
import pulumi

pulumi.export("greeting", "hello from ${PROJECT}, an OCI python program")
