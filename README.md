# oci2vm

English | [简体中文](https://github.com/jimyag/oci2vm/blob/main/README.zh-CN.md)

oci2vm converts container images into bootable virtual machine disk images.

It takes an image from the local Docker daemon, from a registry or built from a Dockerfile, installs a Linux kernel,
a bootloader, cloud-init and an OpenSSH server with the package manager of the image distribution, and writes a
qcow2, raw, vmdk, vdi, vhd, vhdx or qed disk, which boots with QEMU, libvirt, KubeVirt or VirtualBox.
The image entrypoint runs as a service at boot, and the image environment is exported to login shells.

oci2vm is a fork of [d2vm](https://github.com/linka-cloud/d2vm) by Linka Cloud, licensed under the
[Apache License 2.0](https://github.com/jimyag/oci2vm/blob/main/LICENCE). [NOTICE](https://github.com/jimyag/oci2vm/blob/main/NOTICE) lists the attributions.

## How it works

1. oci2vm reads `/etc/os-release` in the image to select the template of its distribution.
2. It builds a temporary image from the image and the [template](https://github.com/jimyag/oci2vm/tree/main/templates), which installs the kernel, the init
   system, the bootloader packages, cloud-init, the OpenSSH server and the network configuration.
3. It writes the temporary image layers to a partitioned raw disk, then sets up `fstab`, the hostname, the name
   servers, the entrypoint service and the bootloader.
4. `qemu-img` converts the raw disk to the output format.

## Quick start

```bash
git clone https://github.com/jimyag/oci2vm && cd oci2vm
make install
oci2vm convert nginx:1.27-alpine -o nginx.qcow2
```

[Complete example](#complete-example) builds an application image, boots it with QEMU and logs in with SSH.

## Installation

Requirements:

- Docker.
- Go and make, to build oci2vm.
- On Linux, root privileges (`sudo`), to set up the loop devices and mount the disk, and these packages:
  util-linux, udev, parted, e2fsprogs, dosfstools, mount, tar, extlinux, qemu-utils, cryptsetup (for LUKS).

On macOS and Windows, or on Linux without root privileges, the `build` and `convert` commands run in the
`ghcr.io/jimyag/oci2vm` image, which already contains these packages. Windows is untested.

No release is published yet. Build oci2vm from source:

```bash
git clone https://github.com/jimyag/oci2vm && cd oci2vm
make install
```

`make install` installs the `oci2vm` binary in `$GOBIN` and builds the `ghcr.io/jimyag/oci2vm` image with the same
version. To run oci2vm with Docker only, build the image and define an alias:

```bash
make docker-build
alias oci2vm="docker run --rm -it --privileged -v /var/run/docker.sock:/var/run/docker.sock -v \$PWD:/oci2vm -w /oci2vm ghcr.io/jimyag/oci2vm:$(make show-version)"
```

With the alias, the build context, the Dockerfile and the output must be in the current directory.

Shell completion is available for bash, zsh, fish and PowerShell:

```bash
source <(oci2vm completion zsh)
```

## Convert an image

```bash
oci2vm convert IMAGE -o disk.qcow2
```

- oci2vm uses the image of the local Docker daemon when it exists, and pulls it from its registry otherwise.
  `--pull` always pulls it, and `--platform linux/arm64` selects another architecture.
- Pull the images of a private registry with `docker pull` first: when oci2vm runs in its image, it cannot use your
  registry credentials.
- An image tarball made by `docker save` or `podman save` keeps the image configuration. Load it first:

  ```bash
  docker load -i app.tar
  oci2vm convert app:latest -o app.qcow2
  ```

- A container filesystem tarball made by `docker export` has no image configuration: oci2vm then installs no
  entrypoint service and exports no environment. Give them back when importing it:

  ```bash
  docker import --change 'ENV GREETING=hello' --change 'ENTRYPOINT ["/app/server"]' app-fs.tar app:fs
  oci2vm convert app:fs -o app.qcow2
  ```

The image must be based on a [supported distribution](#supported-distributions), with `cat` and
`/etc/os-release`: distroless and scratch images cannot be converted.

## Build from a Dockerfile

```bash
oci2vm build -f Dockerfile --build-arg VERSION=1.0 -o disk.qcow2 .
```

`build` runs `docker build` on the context directory, converts the built image like `convert`, then removes it unless
`--keep-cache` is set.

- Multi-stage Dockerfiles work: oci2vm converts the image of the final stage, which must be based on a supported
  distribution. Use for instance `alpine` or `debian:12-slim` instead of `scratch` or distroless as the final stage.
- `build` has no `--target` flag. To convert another stage, build it with Docker, then convert it:

  ```bash
  docker build --target debug -t app:debug .
  oci2vm convert app:debug -o app-debug.qcow2
  ```

- When oci2vm runs in its image, only the context directory is mounted: the Dockerfile must be inside it.

## What the virtual machine includes

- The image `ENTRYPOINT` and `CMD` run at boot as the `oci2vm-entrypoint` service, a systemd unit or, on Alpine,
  an OpenRC service, with the image `ENV`, `WORKDIR` and `USER`. The service runs the generated
  `/usr/local/bin/oci2vm-entrypoint` script and restarts it when it fails. No service is installed when the image has
  no command, or only starts an interactive shell or interpreter, as the distribution and language images do
  (`sh`, `bash`, `python3` or `node` for instance).
- `/etc/profile.d/oci2vm-env.sh` exports the image `ENV` to login shells: SSH sessions, console logins and `su -`.
  `HOME`, `HOSTNAME`, `LOGNAME`, `SHELL`, `TERM` and `USER` keep the values of the login session. Commands run
  without a login shell, such as `ssh host cmd`, and the other services do not read it, except on CentOS, Rocky Linux
  and AlmaLinux, whose `/etc/bashrc` reads `/etc/profile.d`.
- The variables whose names a shell cannot export, such as `my-var`, are skipped by the service and the login shells.
- cloud-init applies at boot the users, SSH keys and user data of the datasource, such as a NoCloud or ConfigDrive
  disk, or a KubeVirt `cloudInitNoCloud` volume:
  - its network configuration is disabled (`/etc/cloud/cloud.cfg.d/99-oci2vm-network.cfg`): the image one is used;
  - `growpart` and `resize2fs` let it extend the root filesystem to the disk size;
  - on Debian, Kali and Ubuntu, its `locale` module is disabled (`99-oci2vm-locale.cfg`): the `locales` package is
    missing from container images;
  - without datasource, ds-identify disables it at boot. Alpine, Debian 9 and CentOS 7 have no ds-identify: there,
    cloud-init only looks for NoCloud and ConfigDrive (`99-oci2vm-datasource.cfg`), instead of waiting minutes for
    the metadata services of the cloud providers.
- An OpenSSH server starts at boot. The host keys created during the build are removed: each virtual machine
  generates its own.
- The network is configured with DHCP on `eth0`:

  | Distribution | Network configuration |
  | --- | --- |
  | Ubuntu 18.04 and later | netplan with systemd-networkd |
  | Debian, Kali, Alpine | ifupdown |
  | CentOS, Rocky Linux, AlmaLinux | NetworkManager |

  `--network-manager` selects `netplan`, `ifupdown` or `none` on Debian, Kali and Ubuntu, and `ifupdown` or `none`
  on Alpine.
- The name servers come from DHCP, unless `--dns` or `--dns-search` set them. `--hostname` and `--add-host` set the
  hostname and `/etc/hosts` entries.
- When oci2vm runs as root, the output disk belongs to the user who ran `sudo`, or else to the owner of the output
  directory.

`--raw` converts the image as it is, without the template: the image must already contain a kernel and an init system,
and gets neither cloud-init nor the OpenSSH server.

## Disk and boot options

| Option | Effect |
| --- | --- |
| `-o disk.EXT` | Output format from the extension: `qcow2`, `qed`, `raw`, `vdi`, `vhd`, `vhdx`, `vmdk`, raw when none |
| `-s 10G` | Disk size, `10G` by default; cloud-init extends the root filesystem to it at boot |
| `-p PASSWORD` | Root password; prefer SSH keys provided by cloud-init |
| `--bootloader` | `syslinux` (default on amd64), `grub-bios`, `grub-efi` (the only one on arm64), or `grub` for BIOS and UEFI |
| `--split-boot`, `--boot-fs`, `--boot-size` | Separate boot partition, ext4 or fat32, `100` MB by default |
| `--luks-password` | Encrypts the root partition with LUKS |
| `--append-to-cmdline` | Extra kernel command line arguments |
| `--force` | Overwrites the output disk |

`grub` and `grub-efi` use a FAT32 boot partition and need CentOS, Rocky Linux or AlmaLinux 9 or later. LUKS is not
available before Ubuntu 20.04 and Debian 10. The [command reference](https://github.com/jimyag/oci2vm/blob/main/docs/content/reference/oci2vm.md) lists all the
flags.

## Run the virtual machine

QEMU, amd64 disk on a Linux host with KVM:

```bash
qemu-system-x86_64 -accel kvm -cpu host -m 1024 -nographic \
  -drive file=disk.qcow2,if=virtio \
  -netdev user,id=n0,hostfwd=tcp::2222-:22 -device virtio-net-pci,netdev=n0
```

QEMU, arm64 disk on a Mac with Apple silicon, where the disk boots with UEFI:

```bash
qemu-system-aarch64 -M virt -accel hvf -cpu host -m 1024 -nographic \
  -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
  -drive file=disk.qcow2,if=virtio \
  -netdev user,id=n0,hostfwd=tcp::2222-:22 -device virtio-net-pci,netdev=n0
```

An amd64 disk using `grub-efi` also needs UEFI firmware, such as `-bios /usr/share/ovmf/OVMF.fd` on Ubuntu.
Attach a cloud-init seed disk with `-drive file=seed.iso,if=virtio,format=raw`.

`oci2vm run qemu`, `oci2vm run vbox` and `oci2vm run hetzner` run a disk with QEMU, VirtualBox or on Hetzner Cloud:
see the [command reference](https://github.com/jimyag/oci2vm/blob/main/docs/content/reference/oci2vm_run.md).

For [KubeVirt](https://kubevirt.io/), `--tag` packages the disk as a
[container disk](https://kubevirt.io/user-guide/storage/disks_and_volumes/#containerdisk) image, and `--push`
pushes it:

```bash
oci2vm convert app:latest -o app.qcow2 --tag registry.example.com/app-disk:latest --push
```

## Complete example

[examples/app](https://github.com/jimyag/oci2vm/tree/main/examples/app) builds a Go web server in a multi-stage Dockerfile, whose final stage is based on
Alpine. The virtual machine runs it as a service, and cloud-init installs your SSH key for root.

1. Replace the key in [examples/app/user-data](https://github.com/jimyag/oci2vm/blob/main/examples/app/user-data) with your public key.
2. Build the disk:

   ```bash
   oci2vm build -o hello.qcow2 examples/app
   ```

3. Create the cloud-init seed disk, on Linux with cloud-image-utils:

   ```bash
   cloud-localds seed.iso examples/app/user-data examples/app/meta-data
   ```

   or on macOS:

   ```bash
   mkdir seed && cp examples/app/user-data examples/app/meta-data seed/
   hdiutil makehybrid -iso -joliet -default-volume-name cidata -o seed.iso seed
   ```

4. Boot it with QEMU, with the seed disk, forwarding the SSH and application ports. On a Linux amd64 host:

   ```bash
   qemu-system-x86_64 -accel kvm -cpu host -m 1024 -nographic \
     -drive file=hello.qcow2,if=virtio -drive file=seed.iso,if=virtio,format=raw \
     -netdev user,id=n0,hostfwd=tcp::2222-:22,hostfwd=tcp::8080-:8080 -device virtio-net-pci,netdev=n0
   ```

   On a Mac with Apple silicon:

   ```bash
   qemu-system-aarch64 -M virt -accel hvf -cpu host -m 1024 -nographic \
     -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
     -drive file=hello.qcow2,if=virtio -drive file=seed.iso,if=virtio,format=raw \
     -netdev user,id=n0,hostfwd=tcp::2222-:22,hostfwd=tcp::8080-:8080 -device virtio-net-pci,netdev=n0
   ```

5. Call the application and log in:

   ```bash
   curl http://localhost:8080   # Hello from oci2vm
   ssh -p 2222 root@localhost
   ```

   In the virtual machine, `rc-service oci2vm-entrypoint status` shows the service, and `echo "$GREETING"` the image
   environment.

The [examples](https://github.com/jimyag/oci2vm/tree/main/examples) directory also contains Dockerfiles for Ubuntu, Debian, Alpine and CentOS Stream:
`make examples` builds all of them.

## Supported distributions

| Distribution | Tested releases | Notes |
| --- | --- | --- |
| Ubuntu | 18.04, 20.04, 22.04, 24.04, 26.04 | |
| Debian | 9, 10, 11, 12, 13 | 9 to 11 install their packages from archive.debian.org |
| Kali | rolling | tested on arm64 only |
| Alpine | 3.16, 3.20 to 3.24 | OpenRC |
| CentOS | 7, 8, Stream 8, Stream 9, Stream 10 | 7, 8 and Stream 8 install their packages from vault.centos.org |
| Rocky Linux | 8, 9, 10 | |
| AlmaLinux | 8, 9, 10 | |

The amd64 disks were tested with KVM and syslinux, and some with grub-efi; the arm64 disks with QEMU on Apple silicon
and grub-efi. RHEL, the other distributions, and the distroless and scratch images are not supported.

## Limitations

- Distroless and scratch images, and final stages based on them, cannot be converted.
- The interactive programs detected to skip the entrypoint service are `sh`, `ash`, `bash`, `dash`, `zsh`, `python`,
  `python3`, `node`, `irb` and `jshell`. Another interactive command, such as `php -a`, becomes a service that stops
  at once.
- On arm64, the kernel command line uses the `ttyS0` serial console, which the QEMU `virt` machine does not have: the
  serial console only shows the kernel messages, so log in with SSH.
- Converting for another architecture than the host one runs the image under emulation: it is slow, and the CentOS 7
  and Stream 8 initramfs generation fails on Apple silicon. The arm64 oci2vm image has no syslinux: amd64 disks
  converted on arm64 hosts need `--bootloader grub-bios` or `grub-efi`.

## Development

| Command | Effect |
| --- | --- |
| `make .build` | Builds the `./oci2vm` binary |
| `make docker-build` | Builds the `ghcr.io/jimyag/oci2vm:VERSION` image |
| `make install` | Both, the binary in `$GOBIN` |
| `make lint` | Runs [golangci-lint](https://golangci-lint.run/) v2, built with Go 1.27 or later, as the CI does |
| `go test -run 'TestDockerImage\|TestImageEnvProfile' .` | Unit tests, without root nor Docker |
| `make tests` | Tests of the root package, with `sudo` and Docker |
| `make test-templates IMAGE=ubuntu:24.04` | Converts an image with its template (`TestConfig`) |
| `make e2e` | Converts and boots images |
| `make examples` | Builds the [examples](https://github.com/jimyag/oci2vm/tree/main/examples) |
| `make cli-docs` | Regenerates the [command reference](https://github.com/jimyag/oci2vm/tree/main/docs/content/reference) |
| `make serve-docs`, `make build-docs` | Serves or builds the documentation site |

`TestConfig`, run by `make test-templates`, removes the images it converts, even those you had pulled before.
Some flag defaults depend on the host: regenerate the command reference on linux/amd64, where the CI checks it with
`make docs-up-to-date`. The CI runs golangci-lint on pushes and pull requests to `main`, and the tests and builds on
pull requests to `main` and on `v*` tags. It publishes neither binaries nor images; on `v*` tags, it deploys the
documentation site to the `gh-pages` branch.

## License

oci2vm is licensed under the [Apache License 2.0](https://github.com/jimyag/oci2vm/blob/main/LICENCE). It derives from
[d2vm](https://github.com/linka-cloud/d2vm) by Linka Cloud, and its run commands are adapted from
[LinuxKit](https://github.com/linuxkit/linuxkit): see [NOTICE](https://github.com/jimyag/oci2vm/blob/main/NOTICE). d2vm was inspired by the
[article](https://iximiuz.com/en/posts/from-docker-container-to-bootable-linux-disk-image/) and the
[docker-to-linux](https://github.com/iximiuz/docker-to-linux) project of [iximiuz](https://github.com/iximiuz).
