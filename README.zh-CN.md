# oci2vm

[English](https://github.com/jimyag/oci2vm/blob/main/README.md) | 简体中文

oci2vm 把容器镜像转换成可以启动的虚拟机磁盘镜像。

它从本地 Docker、镜像仓库或 Dockerfile 获取镜像，用镜像所在发行版的包管理器安装 Linux 内核、引导程序、cloud-init 和
OpenSSH 服务端，再写出 qcow2、raw、vmdk、vdi、vhd、vhdx 或 qed 格式的磁盘，可以用 QEMU、libvirt、KubeVirt 或
VirtualBox 启动。虚拟机启动时，镜像的入口程序作为服务运行，镜像的环境变量会导出到登录 shell。

oci2vm 是 Linka Cloud 的 [d2vm](https://github.com/linka-cloud/d2vm) 的分支，采用
[Apache License 2.0](https://github.com/jimyag/oci2vm/blob/main/LICENCE) 许可证，署名信息见 [NOTICE](https://github.com/jimyag/oci2vm/blob/main/NOTICE)。

## 工作原理

1. 读取镜像里的 `/etc/os-release`，选择对应发行版的模板。
2. 以镜像为基础，按[模板](https://github.com/jimyag/oci2vm/tree/main/templates)构建一个临时镜像，安装内核、init 系统、引导程序的软件包、cloud-init、OpenSSH 服务端和网络配置。
3. 把临时镜像的各层写入分好区的 raw 磁盘，再配置 `fstab`、主机名、DNS、入口服务和引导程序。
4. 用 `qemu-img` 把 raw 磁盘转换成输出格式。

## 快速开始

```bash
git clone https://github.com/jimyag/oci2vm && cd oci2vm
make install
oci2vm convert nginx:1.27-alpine -o nginx.qcow2
```

下文“完整示例”一节演示了如何构建一个应用镜像，再用 QEMU 启动并通过 SSH 登录。

## 安装

需要：

- Docker。
- Go 和 make，用于构建 oci2vm。
- 在 Linux 上直接运行时，需要 root 权限（`sudo`），用来创建 loop 设备和挂载磁盘，并安装以下软件包：util-linux、udev、parted、
  e2fsprogs、dosfstools、mount、tar、extlinux、qemu-utils，使用 LUKS 时还要 cryptsetup。

在 macOS、Windows 上，或者在 Linux 上不以 root 运行时，`build` 和 `convert` 命令会在 `ghcr.io/jimyag/oci2vm`
镜像里执行，这些软件包都已包含在内。Windows 没有测试过。

目前还没有发布版本，需要从源码构建：

```bash
git clone https://github.com/jimyag/oci2vm && cd oci2vm
make install
```

`make install` 会把 `oci2vm` 二进制装到 `$GOBIN`，并构建同一版本的 `ghcr.io/jimyag/oci2vm` 镜像。只想通过 Docker
使用的话，构建镜像后定义一个别名：

```bash
make docker-build
alias oci2vm="docker run --rm -it --privileged -v /var/run/docker.sock:/var/run/docker.sock -v \$PWD:/oci2vm -w /oci2vm ghcr.io/jimyag/oci2vm:$(make show-version)"
```

使用这个别名时，构建上下文、Dockerfile 和输出文件都必须在当前目录下。

支持 bash、zsh、fish 和 PowerShell 的命令补全：

```bash
source <(oci2vm completion zsh)
```

## 转换镜像

```bash
oci2vm convert IMAGE -o disk.qcow2
```

- 本地 Docker 里有这个镜像就直接使用，没有就从镜像仓库拉取。`--pull` 表示总是重新拉取，`--platform linux/arm64`
  用来选择其他架构。
- 私有仓库的镜像要先用 `docker pull` 拉到本地：oci2vm 在自己的镜像里运行时，拿不到你的仓库登录凭据。
- `docker save` 或 `podman save` 导出的镜像 tar 包保留了镜像配置，先导入再转换：

  ```bash
  docker load -i app.tar
  oci2vm convert app:latest -o app.qcow2
  ```

- `docker export` 导出的容器文件系统 tar 包没有镜像配置，oci2vm 不会安装入口服务，也不会导出环境变量。导入时可以补上：

  ```bash
  docker import --change 'ENV GREETING=hello' --change 'ENTRYPOINT ["/app/server"]' app-fs.tar app:fs
  oci2vm convert app:fs -o app.qcow2
  ```

镜像必须基于下文“支持的发行版”里列出的发行版，并且带有 `cat` 和 `/etc/os-release`。distroless 和 scratch 镜像无法转换。

## 从 Dockerfile 构建

```bash
oci2vm build -f Dockerfile --build-arg VERSION=1.0 -o disk.qcow2 .
```

`build` 先对构建上下文目录执行 `docker build`，再像 `convert` 一样转换构建出的镜像。转换结束后，这个镜像会被删除；
加上 `--keep-cache` 时保留。

- 支持多阶段 Dockerfile：oci2vm 转换的是最终阶段的镜像，它必须基于受支持的发行版。最终阶段请使用 `alpine` 或
  `debian:12-slim` 这类镜像，不要用 `scratch` 或 distroless。
- `build` 没有 `--target` 参数。要转换其他阶段，先用 Docker 构建，再转换：

  ```bash
  docker build --target debug -t app:debug .
  oci2vm convert app:debug -o app-debug.qcow2
  ```

- oci2vm 在自己的镜像里运行时只挂载构建上下文目录，所以 Dockerfile 必须放在这个目录里。

## 虚拟机里有什么

- 镜像的 `ENTRYPOINT` 和 `CMD` 在开机时作为 `oci2vm-entrypoint` 服务运行，带上镜像的 `ENV`、`WORKDIR` 和 `USER`。
  这个服务在 systemd 发行版上是一个 unit，在 Alpine 上是一个 OpenRC 服务。服务执行生成的
  `/usr/local/bin/oci2vm-entrypoint` 脚本，失败后会重启。以下两种情况不安装这个服务：
  - 镜像没有命令；
  - 镜像只启动一个交互式 shell 或解释器，例如 `sh`、`bash`、`python3`、`node`。发行版镜像和语言镜像通常都是这样。
- `/etc/profile.d/oci2vm-env.sh` 把镜像的 `ENV` 导出到登录 shell，包括 SSH 会话、控制台登录和 `su -`。
  `HOME`、`HOSTNAME`、`LOGNAME`、`SHELL`、`TERM`、`USER` 保持登录会话自己的值。
  不经过登录 shell 的命令（例如 `ssh host cmd`）和其他服务读不到这个文件。CentOS、Rocky Linux、AlmaLinux 例外，
  它们的 `/etc/bashrc` 会读取 `/etc/profile.d`。
- 名字不是合法 shell 变量名的环境变量（例如 `my-var`），入口服务和登录 shell 都会跳过。
- cloud-init 在开机时从数据源获取用户、SSH 密钥和 user data，例如 NoCloud 或 ConfigDrive 磁盘，KubeVirt 的
  `cloudInitNoCloud` 卷就属于这类：
  - 它的网络配置被关闭（`/etc/cloud/cloud.cfg.d/99-oci2vm-network.cfg`），网络沿用镜像自己的配置；
  - 装好了 `growpart` 和 `resize2fs`，它会把根文件系统扩展到整个磁盘；
  - 在 Debian、Kali 和 Ubuntu 上，它的 `locale` 模块被关闭（`99-oci2vm-locale.cfg`），因为容器镜像里没有 `locales` 软件包；
  - 没有数据源时，ds-identify 会在开机时禁用它。Alpine、Debian 9 和 CentOS 7 没有 ds-identify，所以这三个版本上
    cloud-init 只查找 NoCloud 和 ConfigDrive（`99-oci2vm-datasource.cfg`）。否则它会花上几分钟去等待各云厂商的元数据服务。
- OpenSSH 服务端开机自动启动。构建时生成的主机密钥会被删除，每台虚拟机首次启动时各自生成。
- 网络通过 DHCP 配置在 `eth0` 上：

  | 发行版 | 网络配置 |
  | --- | --- |
  | Ubuntu 18.04 及以上 | netplan 加 systemd-networkd |
  | Debian、Kali、Alpine | ifupdown |
  | CentOS、Rocky Linux、AlmaLinux | NetworkManager |

  `--network-manager` 在 Debian、Kali、Ubuntu 上可以选 `netplan`、`ifupdown` 或 `none`，在 Alpine 上可以选 `ifupdown` 或 `none`。
- DNS 服务器默认来自 DHCP，`--dns` 和 `--dns-search` 可以指定。`--hostname` 和 `--add-host` 分别设置主机名和
  `/etc/hosts` 条目。
- oci2vm 以 root 运行时，输出磁盘归执行 `sudo` 的用户所有；没有 `sudo` 信息时，归输出目录的所有者。

`--raw` 不使用模板，按原样转换镜像。镜像里必须已经有内核和 init 系统，也不会安装 cloud-init 和 OpenSSH 服务端。

## 磁盘和引导选项

| 选项 | 作用 |
| --- | --- |
| `-o disk.EXT` | 按扩展名决定输出格式：`qcow2`、`qed`、`raw`、`vdi`、`vhd`、`vhdx`、`vmdk`，没有扩展名时为 raw |
| `-s 10G` | 磁盘大小，默认 `10G`；开机时 cloud-init 会把根文件系统扩展到这个大小 |
| `-p PASSWORD` | root 密码；更推荐通过 cloud-init 配置 SSH 密钥 |
| `--bootloader` | `syslinux`（amd64 默认）、`grub-bios`、`grub-efi`（arm64 只能用它），以及同时支持 BIOS 和 UEFI 的 `grub` |
| `--split-boot`、`--boot-fs`、`--boot-size` | 独立的启动分区，ext4 或 fat32，默认 `100` MB |
| `--luks-password` | 用 LUKS 加密根分区 |
| `--append-to-cmdline` | 追加内核命令行参数 |
| `--force` | 覆盖已存在的输出磁盘 |

`grub` 和 `grub-efi` 使用 FAT32 启动分区；在 CentOS、Rocky Linux、AlmaLinux 上需要 9 及以上版本。Ubuntu 20.04 和
Debian 10 之前的版本不支持 LUKS。全部参数见[命令参考](https://github.com/jimyag/oci2vm/blob/main/docs/content/reference/oci2vm.md)。

## 启动虚拟机

在带 KVM 的 Linux 主机上用 QEMU 启动 amd64 磁盘：

```bash
qemu-system-x86_64 -accel kvm -cpu host -m 1024 -nographic \
  -drive file=disk.qcow2,if=virtio \
  -netdev user,id=n0,hostfwd=tcp::2222-:22 -device virtio-net-pci,netdev=n0
```

在 Apple 芯片的 Mac 上用 QEMU 启动 arm64 磁盘，这种磁盘通过 UEFI 启动：

```bash
qemu-system-aarch64 -M virt -accel hvf -cpu host -m 1024 -nographic \
  -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
  -drive file=disk.qcow2,if=virtio \
  -netdev user,id=n0,hostfwd=tcp::2222-:22 -device virtio-net-pci,netdev=n0
```

使用 `grub-efi` 的 amd64 磁盘同样需要 UEFI 固件，例如 Ubuntu 上的 `-bios /usr/share/ovmf/OVMF.fd`。
挂载 cloud-init 种子盘时加上 `-drive file=seed.iso,if=virtio,format=raw`。

`oci2vm run qemu`、`oci2vm run vbox`、`oci2vm run hetzner` 可以分别用 QEMU、VirtualBox 运行磁盘，或者把它放到
Hetzner Cloud 上运行，见[命令参考](https://github.com/jimyag/oci2vm/blob/main/docs/content/reference/oci2vm_run.md)。

用于 [KubeVirt](https://kubevirt.io/) 时，`--tag` 把磁盘打包成
[container disk](https://kubevirt.io/user-guide/storage/disks_and_volumes/#containerdisk) 镜像，`--push` 把它推送到镜像仓库：

```bash
oci2vm convert app:latest -o app.qcow2 --tag registry.example.com/app-disk:latest --push
```

## 完整示例

[examples/app](https://github.com/jimyag/oci2vm/tree/main/examples/app) 用多阶段 Dockerfile 构建一个 Go 写的 Web 服务，最终阶段基于 Alpine。虚拟机把它作为
服务运行，cloud-init 为 root 装上你的 SSH 公钥。

1. 把 [examples/app/user-data](https://github.com/jimyag/oci2vm/blob/main/examples/app/user-data) 里的公钥换成你自己的。
2. 构建磁盘：

   ```bash
   oci2vm build -o hello.qcow2 examples/app
   ```

3. 制作 cloud-init 种子盘。在 Linux 上用 cloud-image-utils：

   ```bash
   cloud-localds seed.iso examples/app/user-data examples/app/meta-data
   ```

   在 macOS 上：

   ```bash
   mkdir seed && cp examples/app/user-data examples/app/meta-data seed/
   hdiutil makehybrid -iso -joliet -default-volume-name cidata -o seed.iso seed
   ```

4. 用 QEMU 启动，挂上种子盘，并转发 SSH 和应用的端口。在 Linux amd64 主机上：

   ```bash
   qemu-system-x86_64 -accel kvm -cpu host -m 1024 -nographic \
     -drive file=hello.qcow2,if=virtio -drive file=seed.iso,if=virtio,format=raw \
     -netdev user,id=n0,hostfwd=tcp::2222-:22,hostfwd=tcp::8080-:8080 -device virtio-net-pci,netdev=n0
   ```

   在 Apple 芯片的 Mac 上：

   ```bash
   qemu-system-aarch64 -M virt -accel hvf -cpu host -m 1024 -nographic \
     -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
     -drive file=hello.qcow2,if=virtio -drive file=seed.iso,if=virtio,format=raw \
     -netdev user,id=n0,hostfwd=tcp::2222-:22,hostfwd=tcp::8080-:8080 -device virtio-net-pci,netdev=n0
   ```

5. 访问应用，然后登录：

   ```bash
   curl http://localhost:8080   # Hello from oci2vm
   ssh -p 2222 root@localhost
   ```

   在虚拟机里，`rc-service oci2vm-entrypoint status` 可以查看服务状态，`echo "$GREETING"` 可以看到镜像的环境变量。

[examples](https://github.com/jimyag/oci2vm/tree/main/examples) 目录里还有 Ubuntu、Debian、Alpine 和 CentOS Stream 的 Dockerfile，`make examples` 会把它们全部构建一遍。

## 支持的发行版

| 发行版 | 测试过的版本 | 说明 |
| --- | --- | --- |
| Ubuntu | 18.04、20.04、22.04、24.04、26.04 | |
| Debian | 9、10、11、12、13 | 9 到 11 从 archive.debian.org 安装软件包 |
| Kali | rolling | 只在 arm64 上测试过 |
| Alpine | 3.16、3.20 到 3.24 | 使用 OpenRC |
| CentOS | 7、8、Stream 8、Stream 9、Stream 10 | 7、8 和 Stream 8 从 vault.centos.org 安装软件包 |
| Rocky Linux | 8、9、10 | |
| AlmaLinux | 8、9、10 | |

amd64 磁盘在 KVM 上用 syslinux 测试，其中一部分也测了 grub-efi；arm64 磁盘在 Apple 芯片上用 QEMU 和 grub-efi 测试。
不支持 RHEL、其他发行版，以及 distroless 和 scratch 镜像。

## 限制

- 无法转换 distroless 和 scratch 镜像，也无法转换最终阶段基于它们的多阶段镜像。
- 用来判断“交互式程序、不装入口服务”的程序名有：`sh`、`ash`、`bash`、`dash`、`zsh`、`python`、`python3`、`node`、
  `irb`、`jshell`。其他交互式命令（例如 `php -a`）会被装成服务，启动后立即退出。
- 在 arm64 上，内核命令行指定的串口控制台是 `ttyS0`，QEMU 的 `virt` 机型没有这个设备，所以串口上只能看到内核消息，
  请通过 SSH 登录。
- 转换与主机不同架构的镜像时，镜像在模拟器里运行，速度慢；在 Apple 芯片上，CentOS 7 和 Stream 8 生成 initramfs 会失败。
  arm64 版的 oci2vm 镜像没有 syslinux，在 arm64 主机上转换 amd64 磁盘时，需要指定 `--bootloader grub-bios` 或 `grub-efi`。

## 开发

| 命令 | 作用 |
| --- | --- |
| `make .build` | 构建 `./oci2vm` 二进制 |
| `make docker-build` | 构建 `ghcr.io/jimyag/oci2vm:VERSION` 镜像 |
| `make install` | 二者都构建，二进制装到 `$GOBIN` |
| `make check-fmt`、`make vet` | 检查格式并运行 `go vet`，和 CI 一致 |
| `go test -run 'TestDockerImage\|TestImageEnvProfile' .` | 单元测试，不需要 root 和 Docker |
| `make tests` | 根包的测试，需要 `sudo` 和 Docker |
| `make test-templates IMAGE=ubuntu:24.04` | 用模板转换指定镜像（`TestConfig`） |
| `make e2e` | 转换并启动镜像 |
| `make examples` | 构建 [examples](https://github.com/jimyag/oci2vm/tree/main/examples) 里的示例 |
| `make cli-docs` | 重新生成[命令参考](https://github.com/jimyag/oci2vm/tree/main/docs/content/reference) |
| `make serve-docs`、`make build-docs` | 本地预览或构建文档站 |

`make test-templates` 运行的 `TestConfig` 会删除它转换过的镜像，包括你事先拉取的。部分参数的默认值取决于主机，
所以要在 linux/amd64 上重新生成命令参考，CI 也是在 linux/amd64 上用 `make docs-up-to-date` 检查的。
CI 在向 `main` 提交 PR 和推送 `v*` tag 时运行测试和构建，不发布二进制和镜像；推送 `v*` tag 时会把文档站部署到
`gh-pages` 分支。

## 许可证

oci2vm 采用 [Apache License 2.0](https://github.com/jimyag/oci2vm/blob/main/LICENCE) 许可证。它派生自 Linka Cloud 的 [d2vm](https://github.com/linka-cloud/d2vm)，
run 子命令改编自 [LinuxKit](https://github.com/linuxkit/linuxkit)，详见 [NOTICE](https://github.com/jimyag/oci2vm/blob/main/NOTICE)。d2vm 的思路来自
[iximiuz](https://github.com/iximiuz) 的[文章](https://iximiuz.com/en/posts/from-docker-container-to-bootable-linux-disk-image/)和
[docker-to-linux](https://github.com/iximiuz/docker-to-linux) 项目。
