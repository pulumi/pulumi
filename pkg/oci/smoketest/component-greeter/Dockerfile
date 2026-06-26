# Image for the Node multi-language component (MLC) provider. Same language-image
# shape as the Node program image, but the entrypoint serves the component
# provider gRPC (provider.main) instead of running a program — so from the
# engine's view it is just a provider. Tagged pulumi-provider-greeting:v0.1.0 to
# match the version the consuming program pins, so the container host resolves it
# by the usual convention and runs it as an (own-image) provider.
FROM node:20-alpine
WORKDIR /app
COPY package.json ./
RUN npm install --omit=dev
COPY mlc.js ./
ENTRYPOINT ["node", "/app/mlc.js"]
