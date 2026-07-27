# Node program image for the transform smoke test. Unlike program-node/, this one must run
# THIS BRANCH's @pulumi/pulumi — the whole point is to exercise the SDK's callback-advertise
# behavior, which no released SDK has.
#
# The branch SDK is layered in by OVERWRITING the installed package rather than by installing
# a packed tarball. That is deliberate. `bin/package.json` carries a prerelease version
# (3.x.y-dev.0), and @pulumi/random depends on `@pulumi/pulumi: ^3.142.0`, which semver
# EXCLUDES prereleases from — so a tarball install gives npm a reason to resolve a second,
# nested copy of the SDK, and the program could run its transform through the copy without the
# fix. Installing from the registry first keeps exactly one deduped copy; overwriting its
# contents makes that one copy ours.
#
# The build context must contain sdk-bin/ (a built sdk/nodejs/bin).
FROM node:20-alpine
WORKDIR /workspace
COPY package.json ./
RUN npm install --omit=dev

# Replace the registry copy's contents with this branch's build. node_modules/.bin and the
# dependency tree npm resolved are left untouched.
COPY sdk-bin/ /workspace/node_modules/@pulumi/pulumi/

# The branch SDK's own dependencies are not necessarily a subset of the released one's — it
# currently adds @opentelemetry/exporter-trace-otlp-grpc, which the version npm resolved never
# installed. Resolve the branch manifest INSIDE the package so the extras land in a nested
# node_modules. Deliberately not a second top-level `npm install`: that would resolve the
# PROGRAM's package.json and re-download @pulumi/pulumi from the registry, undoing the overwrite
# above. Today's gap happens to be lazily required, so this is guarding against the next one.
RUN cd /workspace/node_modules/@pulumi/pulumi && npm install --omit=dev --no-package-lock

COPY index.js oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
ENTRYPOINT ["/workspace/oci-bootstrap.sh"]
