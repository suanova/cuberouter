FROM oven/bun:1.4.0@sha256:5ff609364c049b54eb0ff560ec96319729a972078ef2c755d758f0c6ef89c2d6 AS builder

WORKDIR /build/web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY ./web ./
COPY ./VERSION /build/VERSION
RUN DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat /build/VERSION) bun run build

# VitePress documentation sites, served by the app under /docs/user/ and
# /docs/admin/ (see router/docs-router.go). Built in-image so a single
# `docker build` produces everything.
COPY docs-site/user/package.json docs-site/user/bun.lock /build/docs-site/user/
COPY docs-site/admin/package.json docs-site/admin/bun.lock /build/docs-site/admin/
RUN cd /build/docs-site/user && bun install --frozen-lockfile \
    && cd /build/docs-site/admin && bun install --frozen-lockfile
COPY docs-site/user /build/docs-site/user
COPY docs-site/admin /build/docs-site/admin
RUN cd /build/docs-site/user && bun run build \
    && cd /build/docs-site/admin && bun run build

FROM golang:1.26.1-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder2
ENV GO111MODULE=on CGO_ENABLED=0 GOWORK=off

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
# relaykit is a local submodule referenced via replace; its go.mod must be
# present for go mod download to resolve the main module graph.
ADD relaykit/go.mod ./relaykit/go.mod
RUN go mod download

COPY . .
COPY --from=builder /build/web/dist ./web/dist
RUN go build -ldflags "-s -w -X 'github.com/suanova/cuberouter/common.Version=$(cat VERSION)'" -o cube-router

FROM debian:bookworm-slim@sha256:f06537653ac770703bc45b4b113475bd402f451e85223f0f2837acbf89ab020a

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/cube-router /
COPY --from=builder /build/docs-site/user/docs/.vitepress/dist /app/docs/user
COPY --from=builder /build/docs-site/admin/docs/.vitepress/dist /app/docs/admin
# 确保运行用户可读（部分截图文件权限为 600）
RUN chmod -R a+rX /app/docs
COPY LICENSE THIRD-PARTY-LICENSES.md /licenses/
EXPOSE 3000
EXPOSE 443
WORKDIR /data
ENTRYPOINT ["/cube-router"]
