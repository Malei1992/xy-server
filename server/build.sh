#!/usr/bin/env bash
#env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/anbantech/abfuzz/service-go/pkg/constant.GitHash=$(git rev-list -1 HEAD) -X github.com/anbantech/abfuzz/service-go/pkg/constant.GitBranch=$(git branch --show-current) -X 'github.com/anbantech/abfuzz/service-go/pkg/constant.BuildTime=$(date "+%Y-%m-%d %H:%M:%S %Z")'"
set -e

case "$2" in
"linux" | "windows" | "darwin") goos="$2" ;;
"mac") goos="darwin" ;;
"") goos="linux" ;;
*) echo "Unknown 2nd option, GOOS:$2" && exit 1 ;;
esac

case "$3" in
"amd64" | "arm64") goarch="$3" ;;
"") goarch="amd64" ;;
*) echo "Unknown 3rd option, GOARCH:$3" && exit 1 ;;
esac

case "$1" in
"static")
  echo "build pure static binary"
  go generate
  (
    set -x
    env CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -ldflags "-s -w" -o app
  )
  ;;
"")
  echo "build with all features"
  go generate
  (
    set -x
    env CGO_ENABLED=1 GOOS=$goos GOARCH=$goarch CGO_LDFLAGS="-L$(llvm-config --libdir) -Wl,-rpath,$(llvm-config --libdir)" go build -ldflags "-s -w"
  )
  echo "You can compile a pure static binary by using $0 static\n"
  ;;
"-h") echo "get option -h, eg:$0 static" ;;
*) echo "Unknown 1st option: $1" && exit 1 ;;
esac
