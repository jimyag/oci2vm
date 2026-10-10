# Copyright 2022 Linka Cloud  All rights reserved.
# Modifications copyright 2026 jimyag. See the git history for details.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.27 AS builder

WORKDIR /oci2vm

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY . .

RUN make .build

FROM ubuntu:20.04

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
        ca-certificates \
        util-linux \
        udev \
        parted \
        kpartx \
        e2fsprogs \
        dosfstools \
        mount \
        tar \
        "$([ "$(uname -m)" = "x86_64" ] && echo extlinux)" \
        cryptsetup-bin \
        qemu-utils && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY --from=docker:dind /usr/local/bin/docker /usr/local/bin/
# without buildx, docker build falls back to the legacy builder, which fails to
# build images for another platform than the docker host one
COPY --from=docker:dind /usr/local/libexec/docker/cli-plugins/docker-buildx /usr/local/libexec/docker/cli-plugins/

COPY --from=builder /oci2vm/oci2vm /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/oci2vm"]
