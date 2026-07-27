# Python program image for the transform smoke test. Like program-node-transform/, it must
# run THIS BRANCH's SDK — the callback-advertise behavior it exercises is in no released
# pulumi package.
#
# The branch SDK is layered in by OVERWRITING the installed package rather than by installing
# the local tree as a package. Same reasoning as the Node fixture: pip resolves pulumi_random's
# dependency on `pulumi` from the index, and letting it also resolve a local prerelease invites
# a second, mismatched copy. Installing from the index first pins one consistent dependency
# closure; copying the branch source over that one copy makes it ours. Python needs no build
# step for this — lib/pulumi is the shipped package, generated protobuf modules included.
#
# The build context must contain sdk-lib/ (a copy of sdk/python/lib/pulumi) and the
# pulumi-language-python-exec shim, which ships with the CLI rather than the pip package.
FROM python:3.12-slim
WORKDIR /workspace
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Replace the index copy's contents with this branch's source. Resolved dependencies and any
# other installed distribution (pulumi_random) are left untouched.
COPY sdk-lib/ /usr/local/lib/python3.12/site-packages/pulumi/

COPY __main__.py oci-bootstrap.sh pulumi-language-python-exec ./
RUN chmod +x oci-bootstrap.sh pulumi-language-python-exec
ENTRYPOINT ["/workspace/oci-bootstrap.sh"]
