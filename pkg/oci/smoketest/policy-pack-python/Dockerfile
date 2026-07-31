# Policy pack image for the Python analyzer smoke test. A real Python image (python
# + the pulumi-policy SDK via pip, which pulls in pulumi and grpcio) plus the pack's
# source and a marker baked at /policy-marker. The engine runs this image with
# PULUMI_OCI_ROLE=policy-pack; the bootstrap shim boots the pack directly — a Python
# policy pack IS its server (PolicyPack() serves from __init__), so there is no
# run-policy-pack harness on this path. The pack reads /policy-marker inside its
# validation logic, so the marker appearing in a violation proves it ran from THIS
# image.
FROM python:3.12-slim
WORKDIR /policy
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
# Overlay the bind-contract pulumi_policy source on top of the stock install
# (install-then-overwrite — never a second nested copy). The smoke script stages it
# from a pulumi-policy clone; an empty dir (no clone, netns mode) is a no-op and the
# stock SDK serves. version.py is deliberately not staged: the overlay is code, not
# identity (the clone's is an unsubstituted placeholder besides).
COPY policy-sdk-lib/ /usr/local/lib/python3.12/site-packages/pulumi_policy/
COPY __main__.py oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
RUN echo "oci-python-policy-ran-from-its-own-image" > /policy-marker
ENTRYPOINT ["/policy/oci-bootstrap.sh"]
