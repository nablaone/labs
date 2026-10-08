# sitaw: one static binary on an empty image (~11 MB).
#
#   docker build -t sitaw .
#   docker buildx build --platform linux/amd64,linux/arm64 -t sitaw .   # multi-arch
#
# The build runs on the build machine's own platform and cross-compiles
# (CGO is off: the SQLite driver is pure Go), so arm64/amd64 builds are fast.

FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/sitaw ./cmd/sitaw
# Default data dir for plain `docker run`. compose.yaml bind-mounts a host
# directory over it and runs as that directory's owner (SITAW_UID/GID).
RUN mkdir -p /out/data && chown 65532:65532 /out/data

FROM scratch
COPY --from=build /out/sitaw /sitaw
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
ENV SITAW_ADDR=:8080 \
    SITAW_DB=/data/sitaw.db \
    SITAW_IMPORT_JSON=/data/sitaw.json
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD ["/sitaw", "-healthcheck"]
ENTRYPOINT ["/sitaw"]
