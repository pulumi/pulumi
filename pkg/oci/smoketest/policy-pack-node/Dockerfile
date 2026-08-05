# Policy pack image for the analyzer smoke test. A real Node image (node + the
# @pulumi/policy SDK, which pulls in @pulumi/pulumi and its ts-node toolchain) plus
# the pack's TypeScript source and a marker baked at /policy-marker. The bootstrap boots
# the run-policy-pack harness, which compiles index.ts with ts-node — a toolchain present
# here and absent from the engine image. The pack reads /policy-marker inside its
# validation logic, so the marker appearing in a violation proves it ran from THIS image.
ARG PULUMI_SDK_IMAGE=ghcr.io/pulumi-labs/pulumi-pod:latest
FROM ${PULUMI_SDK_IMAGE} AS pulumi-sdk

FROM node:20-alpine
WORKDIR /policy
COPY package.json tsconfig.json ./
RUN npm install
# Overlay the bind-contract @pulumi/policy compiled output baked at /opt/pulumi-sdk/policy in
# the engine image (the smoke run passes --build-arg PULUMI_SDK_IMAGE=<engine>) — exactly as a
# component overlays @pulumi/pulumi. bin only, minus package.json: the overlay is code, not
# identity, and needs no nested install (the stock @pulumi/pulumi it depends on is already
# present). An empty overlay (no policy clone) is a no-op and the stock SDK serves.
COPY --from=pulumi-sdk /opt/pulumi-sdk/policy/ ./node_modules/@pulumi/policy/
COPY index.ts oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
RUN echo "oci-policy-ran-from-its-own-image" > /policy-marker
ENTRYPOINT ["/policy/oci-bootstrap.sh"]
