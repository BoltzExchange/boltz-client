ARG GDK_VERSION
FROM michael1011/gdk-ubuntu-builder:$GDK_VERSION as builder

ARG GDK_VERSION
ARG GDK_ARGS

RUN git clone https://github.com/Blockstream/gdk --depth 1 --branch release_$GDK_VERSION
RUN export PATH="/root/.cargo/bin:$PATH" && cd gdk && ./tools/build.sh --gcc --buildtype release --no-deps-rebuild --external-deps-dir /prebuild/gcc $GDK_ARGS

ENTRYPOINT []
