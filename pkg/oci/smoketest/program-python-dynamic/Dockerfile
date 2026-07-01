# Program image for the Python dynamic-provider smoke test. A real Python language
# image: python + the pulumi SDK via pip, the user program, the SDK's program-exec
# shim (copied from the repo into the build context — it ships with the CLI, not
# the pip package), and a bootstrap shim that boots either the program or the
# dynamic-provider entrypoint (python -m pulumi.dynamic) on PULUMI_OCI_ROLE. A
# marker is baked at /program-marker so the dynamic provider, running from this
# image, can read it to prove it ran welded to the program image.
FROM python:3.12-slim
WORKDIR /workspace
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY __main__.py oci-bootstrap.sh pulumi-language-python-exec ./
RUN chmod +x oci-bootstrap.sh pulumi-language-python-exec
RUN echo "oci-dynamic-welded-to-python-image" > /program-marker
ENTRYPOINT ["/workspace/oci-bootstrap.sh"]
