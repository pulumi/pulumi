# Node program image for the language-image smoke test. Unlike the Go demos
# (a statically-linked binary on alpine), this is a *real* language image: a Node
# base, the @pulumi/pulumi SDK installed via npm, the user program, and an
# entrypoint shim that bootstraps the runtime from PULUMI_* env vars. This is the
# "Dockerfile is the shape declaration" story for an interpreted language — the
# image carries the toolchain (node) and deps, and the contract is just "does the
# container run the program?"
FROM node:20-alpine
WORKDIR /workspace
COPY package.json ./
RUN npm install --omit=dev
COPY index.js oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
ENTRYPOINT ["/workspace/oci-bootstrap.sh"]
