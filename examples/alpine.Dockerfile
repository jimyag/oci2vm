# Alpine with tmux: oci2vm adds the kernel, OpenRC, cloud-init and an OpenSSH server
FROM alpine:3.22

RUN apk add --no-cache tmux
