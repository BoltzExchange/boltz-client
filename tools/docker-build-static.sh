#!/bin/sh

set -eu

target_os="${1:?target os required}"
target_arch="${2:?target arch required}"

case "$target_arch" in
  amd64)
    rust_target=x86_64-unknown-linux-gnu
    tool_prefix=
    ;;
  arm64)
    rust_target=aarch64-unknown-linux-gnu
    tool_prefix=aarch64-linux-gnu-
    ;;
  *)
    echo "unsupported target arch: $target_arch" >&2
    exit 1
    ;;
esac

export GOOS="$target_os"
export GOARCH="$target_arch"
export CGO_ENABLED=1
export GO111MODULE=on
export PKG_CONFIG_ALLOW_CROSS=1
export CC="${tool_prefix}gcc"
export CXX="${tool_prefix}g++"
export AR="${tool_prefix}ar"

exec make static RUST_TARGET="$rust_target"
