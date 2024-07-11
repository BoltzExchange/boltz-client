FROM golang:1.20.2 AS old-builder

# Shallow clone project
RUN git clone --depth=1 --branch v1.2.7 https://github.com/BoltzExchange/boltz-lnd /go/src/github.com/BoltzExchange/boltz-lnd

# Build the binaries
RUN cd /go/src/github.com/BoltzExchange/boltz-lnd \
    && go mod vendor \
    && make build

FROM boltz/boltz-client:latest AS final

VOLUME /root/.boltz

COPY --from=old-builder go/src/github.com/BoltzExchange/boltz-lnd/boltzd /bin/boltzd-old
COPY --from=old-builder go/src/github.com/BoltzExchange/boltz-lnd/boltzcli /bin/boltzcli-old-old

# gRPC and REST ports
EXPOSE 9002 9003

ENTRYPOINT ["sh", "-c", "boltzd || old-boltzd"]
