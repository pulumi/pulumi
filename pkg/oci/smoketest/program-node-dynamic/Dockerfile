# Program image for the dynamic-provider smoke test. A real Node language image
# (node + @pulumi/pulumi + the bootstrap shim), plus a marker baked at
# /program-marker. The dynamic provider runs FROM THIS IMAGE — its entrypoint is
# native to the SDK installed here — and reads the marker to prove it ran welded
# to the program image. The one shim serves both roles, program and dynamic
# provider, selected by PULUMI_OCI_ROLE.
#
# The image runs THIS BRANCH's @pulumi/pulumi: address mode needs the SDK's
# dynamic-provider entrypoint to honor PULUMI_PLUGIN_LISTEN_ADDRESS, which no
# released SDK does. The branch SDK is layered in by OVERWRITING the installed
# package rather than by installing a packed tarball — bin/package.json carries a
# prerelease version, which semver ranges on @pulumi/pulumi EXCLUDE, so a tarball
# install invites a second, nested SDK copy and the provider could serve from the
# copy without the change. Installing from the registry first keeps exactly one
# deduped copy; overwriting its contents makes that one copy ours.
#
# The build context must contain sdk-bin/ (a built sdk/nodejs/bin).
FROM node:20-alpine
WORKDIR /workspace
COPY package.json ./
RUN npm install --omit=dev

# Replace the registry copy's contents with this branch's build, then resolve the
# branch manifest INSIDE the package: the branch SDK's dependencies are not
# necessarily a subset of the released one's. Deliberately not a second top-level
# `npm install` — that would resolve the PROGRAM's manifest and re-download
# @pulumi/pulumi, undoing the overwrite.
COPY sdk-bin/ /workspace/node_modules/@pulumi/pulumi/
RUN cd /workspace/node_modules/@pulumi/pulumi && npm install --omit=dev --no-package-lock

COPY index.js oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
RUN echo "oci-dynamic-welded-to-program-image" > /program-marker
ENTRYPOINT ["/workspace/oci-bootstrap.sh"]
