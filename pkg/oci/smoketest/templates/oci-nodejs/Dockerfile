# The program image: a Node base, your dependencies, your program, and the OCI
# bootstrap entrypoint. This is yours to edit — use any base image you like; the only
# contract is that the container runs the program when started (the bootstrap does that).
# `npm install` here materializes everything in package.json, including any SDK that
# `pulumi package add` declared as a file: dependency.
FROM node:20-alpine
WORKDIR /app
# Bootstrap first: it is the most stable file (it rarely changes), so this layer stays
# cached across program and dependency edits.
COPY oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
COPY package.json ./
RUN npm install --omit=dev
COPY index.js ./
ENTRYPOINT ["/app/oci-bootstrap.sh"]
