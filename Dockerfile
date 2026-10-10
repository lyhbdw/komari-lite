ARG BASE_IMAGE=mirror.gcr.io/library/alpine:3.21
FROM ${BASE_IMAGE}

WORKDIR /app

# Docker buildx 会在构建时自动填充这些变量
ARG TARGETOS
ARG TARGETARCH
ARG OCI_REVISION=
ARG OCI_SOURCE=https://github.com/lyhbdw/komari-lite
ARG OCI_VERSION=

RUN apk add --no-cache ca-certificates curl tzdata

RUN test -n "$OCI_REVISION" && test "$OCI_REVISION" != unknown && test -n "$OCI_VERSION" && test "$OCI_VERSION" != unknown

COPY --chmod=755 komari-${TARGETOS}-${TARGETARCH} /app/komari

ENV GIN_MODE=release
ENV KOMARI_LISTEN=0.0.0.0:25774
ENV SQLITE_TMPDIR=/app/data/.sqlite-tmp
ENV GOMEMLIMIT=200MiB

LABEL org.opencontainers.image.revision="$OCI_REVISION" \
      org.opencontainers.image.source="$OCI_SOURCE" \
      org.opencontainers.image.version="$OCI_VERSION"

EXPOSE 25774

CMD ["/bin/sh", "-c", "mkdir -p \"$SQLITE_TMPDIR\" && exec /app/komari server"]
