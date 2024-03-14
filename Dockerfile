FROM golang:1.21-alpine as go
FROM michael1011/gdk-ubuntu-builder:0.68.2 as builder

ARG GDK_ARGS
RUN git clone https://github.com/Blockstream/gdk --depth 1 --branch release_0.68.2
RUN export PATH="/root/.cargo/bin:$PATH" && cd gdk && ./tools/build.sh --gcc --buildtype release --no-deps-rebuild --external-deps-dir /prebuild/gcc ${GDK_ARGS}

COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:$PATH"

WORKDIR /boltz-client

COPY . ./
RUN cp /root/gdk/gdk/build-gcc/libgreenaddress_full.a /boltz-client/onchain/wallet/lib/

# Build the binaries.
RUN make deps static

FROM scratch AS binaries

COPY --from=builder /boltz-client/boltzd /
COPY --from=builder /boltz-client/boltzcli /

# Start a new, final image.
FROM ubuntu:jammy as final

RUN apt update && apt install ca-certificates -y && rm -rf /var/lib/apt/lists/*

# Root volume for data persistence.
VOLUME /root/.boltz

# Copy binaries.
COPY --from=builder /boltz-client/boltzd /bin/
COPY --from=builder /boltz-client/boltzcli /bin/

# gRPC and REST ports
EXPOSE 9002 9003

ENTRYPOINT ["boltzd"]
