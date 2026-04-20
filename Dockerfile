ARG GO_VERSION
ARG GDK_VERSION
ARG RUST_VERSION
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

FROM boltz/gdk-ubuntu:$GDK_VERSION AS gdk

FROM --platform=$BUILDPLATFORM rust:$RUST_VERSION AS rust
ARG TARGETARCH

RUN rustup component add clippy rustfmt rust-src rust-analyzer && \
    rustup target add wasm32-unknown-unknown && \
    case "${TARGETARCH}" in \
      amd64) rustup target add x86_64-unknown-linux-gnu ;; \
      arm64) rustup target add aarch64-unknown-linux-gnu ;; \
      *) echo "unsupported target arch: ${TARGETARCH}" >&2; exit 1 ;; \
    esac

FROM --platform=$BUILDPLATFORM golang:$GO_VERSION AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /boltz-client

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      build-essential \
      gcc-aarch64-linux-gnu \
      g++-aarch64-linux-gnu \
      libc6-dev-arm64-cross \
      pkg-config && \
    rm -rf /var/lib/apt/lists/*

COPY . ./
COPY --from=rust /usr/local/cargo /usr/local/cargo
COPY --from=rust /usr/local/rustup /usr/local/rustup
COPY --from=gdk / /boltz-client/internal/onchain/wallet/lib/

ENV PATH="/usr/local/cargo/bin:${PATH}" \
    CARGO_HOME="/usr/local/cargo" \
    RUSTUP_HOME="/usr/local/rustup"

# Build the binaries.
RUN --mount=type=cache,id=go-build-${TARGETARCH},target=/root/.cache/go-build \
    --mount=type=cache,id=go-pkg-${TARGETARCH},target=/go/pkg \
    --mount=type=cache,id=bolt12-target-${TARGETARCH},target=/boltz-client/internal/lightning/lib/bolt12/target \
    --mount=type=cache,id=lwk-target-${TARGETARCH},target=/boltz-client/lwk/target \
    --mount=type=cache,id=bdk-target-${TARGETARCH},target=/boltz-client/bdk/target \
    --mount=type=cache,id=cargo-git-${TARGETARCH},target=/usr/local/cargo/git/db \
    --mount=type=cache,id=cargo-registry-${TARGETARCH},target=/usr/local/cargo/registry \
    sh ./tools/docker-build-static.sh "${TARGETOS}" "${TARGETARCH}"

FROM scratch AS binaries

COPY --from=builder /boltz-client/boltzd /
COPY --from=builder /boltz-client/boltzcli /

# Start a new, final image.
FROM ubuntu:noble AS final

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Root volume for data persistence.
VOLUME /root/.boltz

# Copy binaries.
COPY --from=builder /boltz-client/boltzd /bin/
COPY --from=builder /boltz-client/boltzcli /bin/

# gRPC and REST ports
EXPOSE 9002 9003

ENTRYPOINT ["boltzd"]
