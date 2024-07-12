ARG GO_VERSION
ARG GDK_VERSION

FROM boltz/gdk-ubuntu:$GDK_VERSION AS gdk
FROM golang:$GO_VERSION AS builder

WORKDIR /boltz-client

COPY . ./
COPY --from=gdk / /boltz-client/onchain/wallet/lib/

# Build the binaries.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    make deps static

FROM scratch AS binaries

COPY --from=builder /boltz-client/boltzd /
COPY --from=builder /boltz-client/boltzcli /

# Start a new, final image.
FROM ubuntu:jammy AS final

RUN apt update && apt install ca-certificates -y && rm -rf /var/lib/apt/lists/*

# Root volume for data persistence.
VOLUME /root/.boltz

# Copy binaries.
COPY --from=builder /boltz-client/boltzd /bin/
COPY --from=builder /boltz-client/boltzcli /bin/

# gRPC and REST ports
EXPOSE 9002 9003

ENTRYPOINT ["boltzd"]
