ARG GO_VERSION=1.24
ARG NODE_VERSION=22
ARG ALPINE_VERSION=3.22

FROM node:${NODE_VERSION}-alpine AS js-builder

RUN corepack enable

WORKDIR /app

COPY app/package.json app/yarn.lock ./

RUN --mount=type=cache,target=/usr/local/share/.cache/yarn/v6 \
    yarn install --frozen-lockfile --network-timeout 1000000

COPY app/js js/
COPY app/assets assets/
COPY app/vite.config.js .

RUN --mount=type=cache,target=/root/.cache/js-build \
    yarn build

RUN mkdir -p dist && cp assets/* dist/

FROM golang:${GO_VERSION}-alpine AS go-builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Set Go environment variables for faster downloads.
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org
ENV GOCACHE=/root/.cache/go-build
ENV GOMODCACHE=/root/.cache/go-mod

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/root/.cache/go-mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

COPY --from=js-builder /app/dist /app/app/dist

RUN --mount=type=cache,target=/root/.cache/go-mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -buildid=" -o traefik-playground ./cmd

FROM alpine:${ALPINE_VERSION} AS bubblewrap-builder

RUN apk add --no-cache bubblewrap-static

FROM gcr.io/distroless/static:nonroot AS runner

COPY --from=go-builder /app/traefik-playground /app/traefik-playground
COPY --from=bubblewrap-builder /usr/bin/bwrap.static /usr/bin/bwrap

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/traefik-playground", "server"]
