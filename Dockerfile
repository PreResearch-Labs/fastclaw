# Stage 1: Build web UI
FROM node:20-alpine AS web-builder
WORKDIR /app/web
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# Stage 2: Build Go binary
FROM golang:alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache git gcc musl-dev
ENV GOTOOLCHAIN=auto
ENV CGO_ENABLED=1
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=web-builder /app/web/out ./internal/setup/web
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=""
RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o /fastclaw ./cmd/fastclaw

# Stage 3: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=go-builder /fastclaw /usr/local/bin/fastclaw
EXPOSE 18953
ENTRYPOINT ["fastclaw"]
CMD ["gateway"]