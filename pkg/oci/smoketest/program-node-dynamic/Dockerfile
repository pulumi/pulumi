# Program image for the dynamic-provider smoke test. A real Node language image
# (node + @pulumi/pulumi via npm + the bootstrap shim), plus a marker baked at
# /program-marker. The dynamic provider runs FROM THIS IMAGE — its entrypoint is
# native to the SDK installed here — and reads the marker to prove it ran welded
# to the program image. The one shim serves both roles, program and dynamic
# provider, selected by PULUMI_OCI_ROLE.
FROM node:20-alpine
WORKDIR /workspace
COPY package.json ./
RUN npm install --omit=dev
COPY index.js oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
RUN echo "oci-dynamic-welded-to-program-image" > /program-marker
ENTRYPOINT ["/workspace/oci-bootstrap.sh"]
