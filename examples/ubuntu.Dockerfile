# Ubuntu with tmux: oci2vm adds the kernel, cloud-init and an OpenSSH server
FROM ubuntu:24.04

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends tmux && \
    rm -rf /var/lib/apt/lists/*
