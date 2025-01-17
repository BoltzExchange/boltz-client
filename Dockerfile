ARG GO_VERSION
ARG GDK_VERSION
ARG RUST_VERSION

FROM boltz/gdk-ubuntu:$GDK_VERSION AS gdk
FROM rust:$RUST_VERSION AS rust
FROM golang:$GO_VERSION AS builder

WORKDIR /boltz-client

COPY . ./
COPY --from=rust /usr/local/cargo /usr/local/cargo
COPY --from=rust /usr/local/rustup /usr/local/rustup
COPY --from=gdk / /boltz-client/internal/onchain/wallet/lib/

ENV PATH="/usr/local/cargo/bin:${PATH}" \
    CARGO_HOME="/usr/local/cargo" \
    RUSTUP_HOME="/usr/local/rustup"

# Build the binaries.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    --mount=type=cache,target=/boltz-client/lightning/lib/bolt12/target/ \
    --mount=type=cache,target=/usr/local/cargo/git/db \
    --mount=type=cache,target=/usr/local/cargo/registry/ \
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
