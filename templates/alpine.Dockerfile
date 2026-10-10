# Modifications copyright 2026 jimyag. See the git history for details.

{{- if .BootstrapImage }}
FROM {{ .BootstrapImage }} AS packages
RUN apk add --no-cache apk-tools-static
FROM {{ .Image }} AS original
FROM original AS rootfs
{{ else }}
FROM {{ .Image }} AS rootfs
{{ end }}

USER root

{{- if .BootstrapImage }}
COPY --from=packages /sbin/apk.static /tmp/oci2vm-apk
COPY --from=packages /etc/apk/ /etc/apk/
COPY --from=original /etc/ /etc/
RUN /tmp/oci2vm-apk fix --no-cache --upgrade apk-tools && apk --version && rm /tmp/oci2vm-apk
{{ end }}

# BuildKit cannot execute kmod with CAP_SYS_MODULE; depmod needs no capability.
RUN if command -v getcap >/dev/null 2>&1 && command -v setcap >/dev/null 2>&1 && command -v kmod >/dev/null 2>&1; then \
    kmod="$(command -v kmod)" && caps="$(getcap "$kmod")" && \
    if echo "$caps" | grep -q cap_sys_module; then setcap -r "$kmod"; fi; \
  fi

RUN apk add --no-cache \
      util-linux \
      linux-virt \
{{- if ge .Release.VersionID "3.17" }}
      busybox-openrc \
      busybox-mdev-openrc \
      busybox-extras-openrc \
      busybox-mdev-openrc \
{{- else }}
      busybox-initscripts \
{{- end }}
      openrc && \
      find /boot -type l -exec rm {} \;

RUN for s in bootmisc hostname hwclock modules networking swap sysctl urandom syslog; do rc-update add $s boot; done
RUN for s in devfs dmesg hwdrivers mdev; do rc-update add $s sysinit; done

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

{{ if eq .NetworkManager "ifupdown"}}
RUN apk add --no-cache ifupdown-ng
RUN mkdir -p /etc/network && printf '\
auto eth0\n\
allow-hotplug eth0\n\
iface eth0 inet dhcp\n\
' > /etc/network/interfaces
{{ end }}

# the image network configuration is used instead of the cloud-init one,
# growpart and resize2fs let cloud-init extend the root filesystem to the disk size.
# OpenRC runs cloud-init without ds-identify: with no NoCloud or ConfigDrive disk,
# it would wait for the Ec2 and OpenStack metadata services for more than 2 minutes.
RUN apk add --no-cache cloud-init cloud-utils-growpart e2fsprogs-extra && \
    setup-cloud-init && \
    printf 'network:\n  config: disabled\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-network.cfg && \
    printf 'datasource_list: [NoCloud, ConfigDrive, None]\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-datasource.cfg

# the sshd service generates the missing host keys when it starts
RUN apk add --no-cache openssh-server && \
    rm -f /etc/ssh/ssh_host_* && \
    rc-update add sshd default

{{ if .Luks }}
RUN apk add --no-cache cryptsetup && \
    source /etc/mkinitfs/mkinitfs.conf && \
    echo "features=\"${features} cryptsetup\"" > /etc/mkinitfs/mkinitfs.conf && \
    mkinitfs $(ls /lib/modules)
{{- end }}

# we need to keep that at the end, because after it, we can't install packages without error anymore due to grub hooks
{{- if .Grub }}
RUN apk add --no-cache  \
{{- if .GrubBIOS }}
    grub-bios \
{{- end }}
{{- if .GrubEFI }}
    grub-efi \
{{- end }}
    grub || true
{{- end }}

FROM scratch

COPY --from=rootfs / /
