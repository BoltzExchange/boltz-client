ARG GO_VERSION
ARG GDK_VERSION

FROM golang:$GO_VERSION-alpine AS go
FROM boltz/gdk-ubuntu:$GDK_VERSION AS builder

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
