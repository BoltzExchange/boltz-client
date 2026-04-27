#!/bin/sh

set -eu

target_os="${1:?target os required}"
target_arch="${2:?target arch required}"
build_arch="${3:?build arch required}"

case "$target_arch" in
  amd64)
    rust_target=x86_64-unknown-linux-gnu
    cross_prefix=x86_64-linux-gnu-
    ;;
  arm64)
    rust_target=aarch64-unknown-linux-gnu
    cross_prefix=aarch64-linux-gnu-
    ;;
  *)
    echo "unsupported target arch: $target_arch" >&2
    exit 1
    ;;
esac

tool_prefix="$cross_prefix"
if [ "$build_arch" = "$target_arch" ]; then
  tool_prefix=
fi

linker="${tool_prefix}gcc"

export GOOS="$target_os"
export GOARCH="$target_arch"
export PKG_CONFIG_ALLOW_CROSS=1
export CC="$linker"
export CXX="${tool_prefix}g++"
export AR="${tool_prefix}ar"

case "$rust_target" in
  x86_64-unknown-linux-gnu)
    export CARGO_TARGET_X86_64_UNKNOWN_LINUX_GNU_LINKER="$linker"
    ;;
  aarch64-unknown-linux-gnu)
    export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER="$linker"
    ;;
esac

exec make static RUST_TARGET="$rust_target"
