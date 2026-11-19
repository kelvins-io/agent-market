# ---- frontend ----
FROM node:22-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci || npm install
COPY web/ ./
RUN npm run build

# ---- go build ----
FROM golang:1.26-alpine AS build
RUN apk add --no-cache git ca-certificates
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/market ./cmd/market \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/agents ./cmd/agents

# ---- runtime ----
FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 65532 nonroot
WORKDIR /app
COPY --from=build /out/market /app/market
COPY --from=build /out/agents /app/agents
COPY --from=build /src/web/dist /app/web/dist
COPY --from=build /src/configs /app/configs
RUN mkdir -p /data && chown -R nonroot:nonroot /app /data
USER nonroot
ENV HOME=/tmp
EXPOSE 13100 8101 8102 8103 8104
ENTRYPOINT ["/app/market"]
