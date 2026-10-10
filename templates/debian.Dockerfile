# Modifications copyright 2026 jimyag. See the git history for details.

{{- if .BootstrapImage }}
FROM {{ .BootstrapImage }} AS packages
{{- if or (eq .Release.VersionID "9") (eq .Release.VersionID "10") (eq .Release.VersionID "11") }}
RUN echo "deb http://archive.debian.org/debian {{ .Release.VersionCodeName }} main" > /etc/apt/sources.list && \
    echo "deb http://archive.debian.org/debian-security {{ .Release.VersionCodeName }}{{ if eq .Release.VersionID "11" }}-security{{ else }}/updates{{ end }} main" >> /etc/apt/sources.list
{{- end }}
RUN mkdir /packages && cd /packages && apt-get update && apt-get download apt && \
    if command -v gpgv >/dev/null 2>&1; then apt-get download gpgv; else apt-get download sqv; fi
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

{{- if or (eq .Release.VersionID "9") (eq .Release.VersionID "10") (eq .Release.VersionID "11") }}
# stretch, buster and bullseye packages are only available from archive.debian.org
RUN echo "deb http://archive.debian.org/debian {{ .Release.VersionCodeName }} main" > /etc/apt/sources.list && \
    echo "deb http://archive.debian.org/debian-security {{ .Release.VersionCodeName }}{{ if eq .Release.VersionID "11" }}-security{{ else }}/updates{{ end }} main" >> /etc/apt/sources.list
{{- end }}

# initramfs-tools would use the build host swap as resume device, the virtual
# machine waiting for it at boot
RUN mkdir -p /etc/initramfs-tools/conf.d && \
    echo "RESUME=none" > /etc/initramfs-tools/conf.d/resume

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
      linux-image-$(dpkg --print-architecture) && \
      find /boot -type l -exec rm {} \;

RUN ARCH="$([ "$(uname -m)" = "x86_64" ] && echo amd64 || echo arm64)"; \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      systemd-sysv \
      systemd \
      dbus \
      iproute2 \
      isc-dhcp-client \
      iputils-ping

{{- if .Grub }}
RUN DEBIAN_FRONTEND=noninteractive apt install -y grub-common grub2-common
{{- end }}
{{- if .GrubBIOS }}
RUN DEBIAN_FRONTEND=noninteractive apt install -y grub-pc-bin
{{- end }}
{{- if .GrubEFI }}
# the recommended shim-signed would make grub-install add the shim fallback,
# which reboots in a loop looking for the missing EFI/BOOT/shimx64.efi
RUN ARCH="$([ "$(uname -m)" = "x86_64" ] && echo amd64 || echo arm64)"; \
    DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends grub-efi-${ARCH}-bin
{{- end }}

# preset-all enables the units without preset, unlike the distribution:
# systemd-time-wait-sync holds time-sync.target, and cloud-final ordered after it,
# until the clock is synchronized with NTP
RUN systemctl preset-all && \
    if systemctl list-unit-files systemd-time-wait-sync.service | grep -q "^systemd-time-wait-sync.service"; then \
      systemctl disable systemd-time-wait-sync.service; \
    fi

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

{{ if eq .NetworkManager "netplan" }}
# systemd-resolved, a separate package since bookworm, provides the DHCP name servers
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
{{- if eq .Release.VersionID "9" }}
# stretch ifupdown2 networking.service is ordered after sysinit.target, which
# creates an ordering cycle with cloud-init.service
RUN apt install -y ifupdown
{{- else }}
RUN if [ -z "$(apt-cache madison ifupdown2 2> /dev/nul)" ]; then apt install -y ifupdown; else apt install -y ifupdown2; fi
{{- end }}
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
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends cloud-init cloud-guest-utils e2fsprogs && \
    printf 'network:\n  config: disabled\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-network.cfg && \
    printf 'locale: false\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-locale.cfg
{{- if eq .Release.VersionID "9" }}
# the stretch cloud-init generator does not run ds-identify: with no NoCloud or
# ConfigDrive disk, cloud-init would wait for the cloud metadata services
RUN printf 'datasource_list: [NoCloud, ConfigDrive, None]\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-datasource.cfg
{{- end }}

# remove the build host keys so that each virtual machine generates its own:
# ssh-keygen -A must run before the "sshd -t" configuration check.
# Kali does not enable the network services it installs.
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends openssh-server && \
    rm -f /etc/ssh/ssh_host_* && \
    mkdir -p /etc/systemd/system/ssh.service.d && \
    printf '[Service]\nExecStartPre=\nExecStartPre=/usr/bin/ssh-keygen -A\nExecStartPre=/usr/sbin/sshd -t\n' > /etc/systemd/system/ssh.service.d/oci2vm-host-keys.conf && \
    systemctl enable ssh.service


{{- if .Luks }}
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends cryptsetup-initramfs && \
    echo "CRYPTSETUP=y" >> /etc/cryptsetup-initramfs/conf-hook && \
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
