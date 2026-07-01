# The program image: a Node base, your dependencies, your program, and the OCI
# bootstrap entrypoint. This is yours to edit — use any base image you like; the
# contract is that the container runs the program when started (the bootstrap does
# that), rooted at /workspace. The pod mounts the one shared workspace volume at
# /workspace and runs the program there, so build into /workspace: it is the working
# directory where a provider reads the assets and file paths your program produces.
# (A plugin's own code lives at /plugin, kept distinct so the workspace never shadows
# it.) `npm install` here materializes everything in package.json, including any SDK
# that `pulumi package add` declared as a file: dependency.
FROM node:20-alpine
WORKDIR /workspace
# Bootstrap first: it is the most stable file (it rarely changes), so this layer stays
# cached across program and dependency edits.
COPY oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
# Deps + any locally-generated SDKs, before the program. `pulumi package add` writes SDKs
# under sdks/ and a `file:` dependency into package.json; copying sdks/ here lets `npm
# install` resolve them. sdks/ is shipped (with .gitkeep) so this COPY is a no-op when no
# SDK has been added. This layer is invalidated only when deps/SDKs change — not on every
# program edit.
COPY package.json ./
COPY sdks ./sdks
RUN npm install --omit=dev
# Bake a best-effort manifest of the Pulumi provider packages these deps require,
# read from their installed Pulumi plugin metadata, to a well-known path OUTSIDE
# /workspace (which the pod shadows with the shared volume at runtime). The OCI
# engine reads it to pre-fetch provider images; lazy discovery stays authoritative,
# so this is an optimization, never a gate. Sits with the deps layer (before the
# program copy) so it is not rebuilt on every program edit.
COPY oci-required-packages.cjs ./
RUN node oci-required-packages.cjs /workspace/node_modules /pulumi/required-packages.json
COPY index.js ./
ENTRYPOINT ["/workspace/oci-bootstrap.sh"]
