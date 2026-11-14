FROM node:20-alpine

WORKDIR /app

# Copy the locally pre-built Nuxt output
COPY .output ./.output

ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=8816 \
    NITRO_HOST=0.0.0.0 \
    NITRO_PORT=8816

EXPOSE 8816

CMD ["node", ".output/server/index.mjs"]
