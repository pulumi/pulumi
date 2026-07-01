# Image for the Node multi-language component (MLC) provider. Same language-image
# shape as the Node program image, but the entrypoint serves the component
# provider gRPC (provider.main) instead of running a program — so from the
# engine's view it is just a provider. Tagged pulumi-provider-greeting:v0.1.0 to
# match the version the consuming program pins, so the container host resolves it
# by the usual convention and runs it as an (own-image) provider.
#
# The component's own code lives at /plugin, NOT the shared workspace path
# (/workspace, oci.WorkspaceMountPath). The container host mounts the per-pod
# workspace volume at /workspace with the working directory set there; if this
# image put its code at that path, Docker's empty-volume seeding would shadow
# mlc.js with the program's workspace. Keeping own-code (/plugin) distinct from
# the shared workspace (/workspace) is what lets the program and this MLC run in
# one pod without stepping on each other. /plugin matches the provider-binary
# convention (Dockerfile.provider stages at /plugin) and PulumiPlugin.yaml.
FROM node:20-alpine
WORKDIR /plugin
COPY package.json ./
RUN npm install --omit=dev
COPY mlc.js ./
ENTRYPOINT ["node", "/plugin/mlc.js"]
