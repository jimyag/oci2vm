# CentOS Stream with tmux: oci2vm adds the kernel, cloud-init and an OpenSSH server
FROM quay.io/centos/centos:stream10

RUN dnf install -y tmux && \
    dnf clean all
