# The program image: a Node base, your dependencies, your program, and the OCI
# bootstrap entrypoint. This is yours to edit — use any base image you like; the only
# contract is that the container runs the program when started (the bootstrap does that).
FROM node:20-alpine
WORKDIR /app
COPY package.json ./
RUN npm install --omit=dev
COPY index.js oci-bootstrap.sh ./
RUN chmod +x oci-bootstrap.sh
ENTRYPOINT ["/app/oci-bootstrap.sh"]
