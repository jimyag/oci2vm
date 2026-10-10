# Modifications copyright 2026 jimyag. See the git history for details.

{{- if .BootstrapImage }}
FROM {{ .BootstrapImage }} AS packages
RUN mkdir /packages && cd /packages && apt-get update && apt-get download apt gpgv
{{ end }}
FROM {{ .Image }} AS rootfs

USER root

{{- if .BootstrapImage }}
COPY --from=packages /packages/ /tmp/oci2vm-packages/
RUN dpkg -i /tmp/oci2vm-packages/*.deb && rm -r /tmp/oci2vm-packages
{{ end }}

# BuildKit cannot execute kmod with CAP_SYS_MODULE; depmod needs no capability.
RUN if command -v getcap >/dev/null 2>&1 && command -v setcap >/dev/null 2>&1 && command -v kmod >/dev/null 2>&1; then \
    kmod="$(command -v kmod)" && caps="$(getcap "$kmod")" && \
    if echo "$caps" | grep -q cap_sys_module; then setcap -r "$kmod"; fi; \
  fi

# initramfs-tools would use the build host swap as resume device, the virtual
# machine waiting for it at boot
RUN mkdir -p /etc/initramfs-tools/conf.d && \
  echo "RESUME=none" > /etc/initramfs-tools/conf.d/resume

RUN ARCH="$([ "$(uname -m)" = "x86_64" ] && echo amd64 || echo arm64)"; \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
  linux-image-virtual \
  initramfs-tools \
  systemd-sysv \
  systemd \
{{- if .Grub }}
  grub-common \
  grub2-common \
{{- end }}
{{- if .GrubBIOS }}
  grub-pc-bin \
{{- end }}
{{- if .GrubEFI }}
  grub-efi-${ARCH}-bin \
{{- end }}
  dbus \
  isc-dhcp-client \
  iproute2 \
  iputils-ping && \
  find /boot -type l -exec rm {} \;

{{ if gt .Release.VersionID "16.04" }}
# preset-all enables the units without preset, unlike the distribution:
# systemd-time-wait-sync holds time-sync.target, and cloud-final ordered after it,
# until the clock is synchronized with NTP, and systemd 237 enables serial-getty@
# without instance, which waits 90 seconds for a missing device
RUN systemctl preset-all && \
  rm -f /etc/systemd/system/getty.target.wants/serial-getty@.service && \
  if systemctl list-unit-files systemd-time-wait-sync.service | grep -q "^systemd-time-wait-sync.service"; then \
    systemctl disable systemd-time-wait-sync.service; \
  fi
{{ end }}

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

{{ if eq .NetworkManager "netplan" }}
# systemd-resolved, a separate package since 23.04, provides the DHCP name servers
RUN apt install -y netplan.io && \
  if apt-cache show systemd-resolved > /dev/null 2>&1; then apt install -y systemd-resolved; fi
RUN mkdir -p /etc/netplan && printf '\
network:\n\
  version: 2\n\
  renderer: networkd\n\
  ethernets:\n\
    eth0:\n\
      dhcp4: true\n\
      dhcp-identifier: mac\n\
' > /etc/netplan/00-netcfg.yaml
{{ else if eq .NetworkManager "ifupdown"}}
RUN if [ -z "$(apt-cache madison ifupdown-ng 2> /dev/nul)" ]; then apt install -y ifupdown; else apt install -y ifupdown-ng; fi
RUN mkdir -p /etc/network && printf '\
auto eth0\n\
allow-hotplug eth0\n\
iface eth0 inet dhcp\n\
' > /etc/network/interfaces
# systemd-networkd, enabled by preset-all, does not manage eth0: its wait-online
# service would delay network-online.target until its 2 minutes timeout
RUN for u in systemd-networkd.socket systemd-networkd.service systemd-networkd-wait-online.service; do \
    if systemctl list-unit-files "$u" | grep -q "^$u"; then systemctl disable "$u"; fi; \
  done
{{ end }}

# the image network configuration is used instead of the cloud-init one,
# growpart and resize2fs let cloud-init extend the root filesystem to the disk size,
# the locale module is skipped: it needs the locales package, missing from container images
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends cloud-init cloud-guest-utils e2fsprogs lsb-release && \
  printf 'network:\n  config: disabled\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-network.cfg && \
  printf 'locale: false\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-locale.cfg

# remove the build host keys so that each virtual machine generates its own:
# ssh-keygen -A must run before the "sshd -t" configuration check
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends openssh-server && \
  rm -f /etc/ssh/ssh_host_* && \
  mkdir -p /etc/systemd/system/ssh.service.d && \
  printf '[Service]\nExecStartPre=\nExecStartPre=/usr/bin/ssh-keygen -A\nExecStartPre=/usr/sbin/sshd -t\n' > /etc/systemd/system/ssh.service.d/oci2vm-host-keys.conf

{{- if .Luks }}
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends cryptsetup-initramfs && \
    update-initramfs -u -v
{{- end }}

# needs to be after update-initramfs
{{- if not .Grub }}
RUN mv $(ls -t /boot/vmlinuz-* | head -n 1) /boot/vmlinuz && \
      mv $(ls -t /boot/initrd.img-* | head -n 1) /boot/initrd.img
{{- end }}

RUN apt-get clean && \
    rm -rf /var/lib/apt/lists/*

FROM scratch

COPY --from=rootfs / /
