# syntax=docker/dockerfile:1
FROM node:22-bookworm-slim AS build
WORKDIR /build
ENV NEXT_TELEMETRY_DISABLED=1 EPCERP_API_URL=http://api:8001/api/v1
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-bookworm-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 HOSTNAME=0.0.0.0 PORT=3000
COPY --from=build --chown=node:node /build/.next/standalone ./
COPY --from=build --chown=node:node /build/.next/static ./.next/static
COPY --from=build --chown=node:node /build/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]
