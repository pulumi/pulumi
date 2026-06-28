# The program image: a Python base, your dependencies, your program, the OCI bootstrap
# entrypoint, and the SDK's program-exec shim. This is yours to edit — use any base image
# you like; the only contract is that the container runs the program when started.
#
# NOTE (vendored shim): pulumi-language-python-exec is the Python run harness. Unlike
# Node's harness (which lives in the @pulumi/pulumi npm package), Python's ships with the
# Pulumi CLI, NOT the pip package — so we vendor a copy into the template. That is the wart
# to fix upstream: ship pulumi-language-python-exec in the `pulumi` pip package, which would
# make Python symmetric with Node (harness in the package) and let this template drop the
# vendored copy. Until then, keep it in sync with your CLI version.
FROM python:3.12-slim
WORKDIR /app
# Bootstrap + the vendored run harness first: they are the most stable files (they rarely
# change), so this layer stays cached across program and dependency edits.
COPY oci-bootstrap.sh pulumi-language-python-exec ./
RUN chmod +x oci-bootstrap.sh pulumi-language-python-exec
# Deps + any locally-generated SDKs, before the program. `pulumi package add` writes SDKs
# under sdks/ and a local requirement into requirements.txt; copying sdks/ here lets `pip
# install` resolve them. sdks/ is shipped (with .gitkeep) so this COPY is a no-op until an
# SDK is added. Invalidated only when deps/SDKs change, not on every program edit.
COPY requirements.txt ./
COPY sdks ./sdks
RUN pip install --no-cache-dir -r requirements.txt
COPY __main__.py ./
ENTRYPOINT ["/app/oci-bootstrap.sh"]
