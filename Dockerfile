# Static CGO_ENABLED=0 cross-compile on the DHI Go toolchain, scratch runtime,
# nonroot. go.mod pins go 1.21; -mod=mod lets the current toolchain resolve as
# needed while go.sum still pins every dependency version.
FROM --platform=$BUILDPLATFORM dhi.io/golang:1.27.0-alpine-dev@sha256:19b188d9533719b78143b1a6d64a064a39a614cbb0275274855829f3c49abe6c AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -mod=mod -trimpath -ldflags='-w -s' -o /out/jellyseerr-exporter .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/jellyseerr-exporter /jellyseerr-exporter

USER 65532:65532
EXPOSE 9850
ENTRYPOINT ["/jellyseerr-exporter"]
