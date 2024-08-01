ARG GDK_VERSION
FROM michael1011/gdk-ubuntu-builder:$GDK_VERSION AS builder

ARG GDK_VERSION
ARG GDK_ARGS

RUN git clone https://github.com/Blockstream/gdk --depth 1 --branch release_$GDK_VERSION
RUN export PATH="/root/.cargo/bin:$PATH" && cd gdk && \
    ./tools/build.sh --gcc --buildtype release --no-deps-rebuild --external-deps-dir /prebuild/gcc $GDK_ARGS && \
    ./tools/build.sh --gcc --buildtype release --no-deps-rebuild --external-deps-dir /prebuild/gcc --static $GDK_ARGS

FROM scratch AS final

COPY --from=builder /root/gdk/gdk/build-gcc/src/libgreen_gdk.a /
COPY --from=builder /root/gdk/gdk/build-gcc/src/libgreen_gdk.so /

ENTRYPOINT []
