#!/usr/bin/env bash
# Dockerfile用于快速构建，使用本地机器构建二进制，并copy到镜像中
# DockerfileSimple-action用于多平台构建，使用docker容器分阶段构建镜像
set -e

case "$1" in
"amd64" | "arm64") ARCH="$1" ;;
"") ARCH="amd64" ;;
*) echo "Unknown 1st option, ARCH:$1" && exit 1 ;;
esac

# update this tag when you build a new image.(x86_64-linux-4.8.4/arm64-linux-4.8.4)
#./build.sh static linux "$ARCH"
#(cd "$cmdDir"/absearch-server && ./xbuild.sh linux "$ARCH")
docker build -f Dockerfile --platform linux/"$ARCH"  -t "xy-server:latest" .
