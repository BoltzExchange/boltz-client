FROM golang:1.20.2-alpine3.17 as builder

# Install dependencies.
RUN apk add --no-cache --update \
    alpine-sdk \
    git \
    make \
    gcc

# Shallow clone project.
RUN git clone --depth=1 https://github.com/BoltzExchange/boltz-lnd /go/src/github.com/BoltzExchange/boltz-lnd

# Build the binaries.
RUN cd /go/src/github.com/BoltzExchange/boltz-lnd \
    && go mod vendor \
    && make build

# Start a new, final image.
FROM alpine:3.17 as final

# Root volume for data persistence.
VOLUME /root/.boltz-lnd

# Copy binaries.
COPY --from=builder /go/src/github.com/BoltzExchange/boltz-lnd/boltzd /bin/
COPY --from=builder /go/src/github.com/BoltzExchange/boltz-lnd/boltzcli /bin/

# gRPC and REST ports
EXPOSE 9002 9003

ENTRYPOINT ["boltzd"]
