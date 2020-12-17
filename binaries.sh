#!/bin/bash

function print() {
  echo -e " \e[0;32m${1}\e[0m"
}

function cc_version() {
  case $1 in
    "linux")
      case $2 in
        "amd64")
          echo "gcc"
          ;;

        "arm")
          echo "arm-linux-gnueabi-gcc"
          ;;

        "arm64")
          echo "aarch64-linux-gnu-gcc"
          ;;
      esac

      ;;

      "windows")
        case $2 in
          "amd64")
            echo "x86_64-w64-mingw32-gcc"
            ;;
        esac
  esac
}

mkdir -p binaries

for build_system in "$@"; do
  print "Building binaries for $build_system"

  os=$(echo "$build_system" | cut -f1 -d-)
  arch=$(echo "$build_system" | cut -f2 -d-)

  env CGO_ENABLED=1 CC="$(cc_version "$os" "$arch")" GOOS="$os" GOARCH="$arch" make build

  print "Moving binaries for $build_system"

  destinationPath=binaries/$build_system/
  mkdir -p "$destinationPath"

  if [[ $os == "windows" ]]; then
      mv boltzd "$destinationPath"/boltzd.exe
      mv boltzcli "$destinationPath"/boltzcli.exe
  else
      mv boltzd "$destinationPath"/boltzd
      mv boltzcli "$destinationPath"/boltzcli
  fi
done
