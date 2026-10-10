# Modifications copyright 2026 jimyag. See the git history for details.

FROM {{ .Image }} AS rootfs

USER root

{{ if and (eq .Release.ID "centos") (le (atoi .Release.VersionID) 8) }}
RUN sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*
{{ end }}

# See https://bugzilla.redhat.com/show_bug.cgi?id=1917213
RUN yum install -y \
    kernel \
    systemd \
    NetworkManager \
    e2fsprogs \
    sudo && \
    systemctl enable NetworkManager && \
    systemctl unmask systemd-remount-fs.service && \
    systemctl unmask getty.target && \
    mkdir -p /boot && \
    find /boot -type l -exec rm {} \;

{{- if eq .Release.VersionID "7" }}
# the legacy network service fails without this file, NetworkManager still
# configures the network
RUN touch /etc/sysconfig/network
{{- end }}

{{- if .GrubBIOS }}
RUN yum install -y grub2
{{- end }}
{{- if .GrubEFI }}
RUN ARCH="$([ "$(uname -m)" = "x86_64" ] && echo x64 || echo aa64)"; \
    yum install -y grub2 grub2-efi-${ARCH} grub2-efi-${ARCH}-modules
{{- end }}

{{ if .Luks }}
RUN yum install -y cryptsetup && \
    dracut --no-hostonly --regenerate-all --force --install="/usr/sbin/cryptsetup"
{{ else }}
RUN dracut --no-hostonly --regenerate-all --force
{{ end }}

{{ if .Password }}RUN echo "root:{{ .Password }}" | chpasswd {{ end }}

# the image network configuration is used instead of the cloud-init one,
# growpart lets cloud-init extend the root partition to the disk size,
# hostname and service, missing from some images, are used to set the
# instance hostname and to restart sshd
RUN yum install -y cloud-init cloud-utils-growpart hostname /usr/sbin/service && \
    printf 'network:\n  config: disabled\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-network.cfg
{{- if eq .Release.VersionID "7" }}
# the CentOS 7 cloud-init generator looks for ds-identify in /usr/libexec, where
# the package does not install it: with no NoCloud or ConfigDrive disk,
# cloud-init would wait for the cloud metadata services
RUN printf 'datasource_list: [NoCloud, ConfigDrive, None]\n' > /etc/cloud/cloud.cfg.d/99-oci2vm-datasource.cfg
{{- end }}

# sshd-keygen generates the missing host keys when sshd starts
RUN yum install -y openssh-server && \
    rm -f /etc/ssh/ssh_host_* && \
    systemctl enable sshd

{{- if .Grub }}
RUN set -e; \
    for linux in /usr/lib/modules/*/vmlinuz; do \
      [ -e "$linux" ] || continue; \
      kver="$(basename "$(dirname "$linux")")"; \
      cp "/usr/lib/modules/${kver}/vmlinuz" "/boot/vmlinuz-${kver}"; \
    done
RUN set -e; \
    for linux in /boot/*/*/linux; do \
      [ -e "$linux" ] || continue; \
      dir="$(dirname "$linux")"; \
      kver="$(basename "$dir")"; \
      initrd="${dir}/initrd"; \
      [ -e "$initrd" ] || continue; \
      mv "${linux}" "/boot/vmlinuz-${kver}"; \
      mv "${initrd}" "/boot/initramfs-${kver}.img"; \
    done
{{- else }}
RUN cd /boot && \
      mv $(find / -name 'vmlinuz*') /boot/vmlinuz && \
      mv $(find . -name 'initramfs-*.img' -o -name initrd) /boot/initrd.img
{{- end }}

RUN yum clean all && \
    rm -rf /var/cache/yum

FROM scratch

COPY --from=rootfs / /
