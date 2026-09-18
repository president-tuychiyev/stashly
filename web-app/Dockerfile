# Server amd64 — image ham amd64 bo'lishi shart (arm64 mashinada yig'ilsa ham).
FROM --platform=linux/amd64 node:20-alpine

WORKDIR /app

# Copy the locally pre-built Nuxt output
COPY .output ./.output

ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=8808 \
    NITRO_HOST=0.0.0.0 \
    NITRO_PORT=8808

EXPOSE 8808

CMD ["node", ".output/server/index.mjs"]
