# Next.js in standalone mode: `next build` traces exactly the files the server
# needs, so the runtime image carries no node_modules tree and no source.

FROM docker.io/library/node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

FROM docker.io/library/node:20-alpine AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM docker.io/library/node:20-alpine
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
RUN adduser -D -u 10001 astralis
COPY --from=build --chown=astralis /app/.next/standalone ./
COPY --from=build --chown=astralis /app/.next/static ./.next/static
USER astralis
EXPOSE 3000
CMD ["node", "server.js"]
