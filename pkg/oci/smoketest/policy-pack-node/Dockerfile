# Policy pack image for the analyzer smoke test. A real Node image (node + the
# @pulumi/policy SDK, which pulls in @pulumi/pulumi and its ts-node toolchain) plus
# the pack's TypeScript source and a marker baked at /policy-marker. The engine runs
# this image with PULUMI_OCI_ROLE=policy-pack; the bootstrap shim boots the
# run-policy-pack harness, which compiles index.ts with ts-node — a toolchain present
# here and absent from the engine image. The pack reads /policy-marker inside its
# validation logic, so the marker appearing in a violation proves it ran from THIS
# image.
FROM node:20-alpine
WORKDIR /policy
COPY package.json tsconfig.json ./
RUN npm install
COPY index.ts oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
RUN echo "oci-policy-ran-from-its-own-image" > /policy-marker
ENTRYPOINT ["/policy/oci-bootstrap.sh"]
