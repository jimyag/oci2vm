// Modifications copyright 2026 jimyag. See the git history for details.

package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.linka.cloud/console"

	"github.com/jimyag/oci2vm/pkg/qemu_img"
)

var (
	VboxCmd = &cobra.Command{
		Use:   "vbox [options] image-path",
		Short: "Run the virtual machine image with Virtualbox",
		Args:  cobra.ExactArgs(1),
		Run:   Vbox,
	}

	vboxmanageFlag string
	name           string
	networks       VBNetworks
)

func init() {
	flags := VboxCmd.Flags()
	// Display flags
	flags.Bool("gui", false, "Show the VM GUI")

	// vbox options
	flags.StringVar(&vboxmanageFlag, "vboxmanage", "VBoxManage", "VBoxManage binary to use")
	flags.StringVar(&name, "name", "oci2vm", "Name of the Virtualbox VM")

	// Paths and settings for disks
	flags.Var(&disks, "disk", "Disk config, may be repeated. [file=]path[,size=1G][,format=raw]")

	// VM configuration
	flags.Uint("cpus", 1, "Number of CPUs")
	flags.Uint("mem", 1024, "Amount of memory in MB")

	// networking
	flags.Var(&networks, "networking", "Network config, may be repeated. [type=](null|nat|bridged|intnet|hostonly|generic|natnetwork[<devicename>])[,[bridge|host]adapter=<interface>]")
}

func Vbox(cmd *cobra.Command, args []string) {
	// only this command: in init, the check stopped every command on Windows
	if runtime.GOOS == "windows" {
		logrus.Fatalf("TODO: Windows is not yet supported")
	}
	path := args[0]
	if err := vbox(cmd.Context(), path); err != nil {
		logrus.Fatal(err)
	}
}

func vbox(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	vboxmanage, err := exec.LookPath(vboxmanageFlag)
	if err != nil {
		return fmt.Errorf("cannot find management binary %s: %w", vboxmanageFlag, err)
	}
	i, err := qemu_img.Info(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to get image info: %w", err)
	}
	if i.Format != "vdi" {
		logrus.Warnf("image format is %s, expected vdi", i.Format)
		vdi := filepath.Join(os.TempDir(), "oci2vm", "run", filepath.Base(path)+".vdi")
		if err := os.MkdirAll(filepath.Dir(vdi), 0o755); err != nil {
			return err
		}
		defer os.RemoveAll(vdi)
		logrus.Infof("converting image to raw: %s", vdi)
		if err := qemu_img.Convert(ctx, "vdi", path, vdi); err != nil {
			return err
		}
		path = vdi
	}

	// remove machine in case it already exists
	cleanup(vboxmanage, name, false)

	out, err := manage(vboxmanage, "createvm", "--name", name, "--register")
	if err != nil {
		return fmt.Errorf("createvm error: %w\n%s", err, out)
	}

	out, err = manage(vboxmanage, "modifyvm", name, "--acpi", "on")
	if err != nil {
		return fmt.Errorf("modifyvm --acpi error: %w\n%s", err, out)
	}

	out, err = manage(vboxmanage, "modifyvm", name, "--memory", fmt.Sprintf("%d", mem))
	if err != nil {
		return fmt.Errorf("modifyvm --memory error: %w\n%s", err, out)
	}

	out, err = manage(vboxmanage, "modifyvm", name, "--cpus", fmt.Sprintf("%d", cpus))
	if err != nil {
		return fmt.Errorf("modifyvm --cpus error: %w\n%s", err, out)
	}

	out, err = manage(vboxmanage, "modifyvm", name, "--firmware", "bios")
	if err != nil {
		return fmt.Errorf("modifyvm --firmware error: %w\n%s", err, out)
	}

	// set up serial console
	out, err = manage(vboxmanage, "modifyvm", name, "--uart1", "0x3F8", "4")
	if err != nil {
		return fmt.Errorf("modifyvm --uart error: %w\n%s", err, out)
	}

	consolePath := filepath.Join(os.TempDir(), "oci2vm-vb", name, "console")
	if err := os.MkdirAll(filepath.Dir(consolePath), os.ModePerm); err != nil {
		return fmt.Errorf("mkir %s: %w", consolePath, err)
	}
	// TODO use a named pipe on Windows
	if runtime.GOOS != "windows" {
		consolePath, err = filepath.Abs(consolePath)
		if err != nil {
			return fmt.Errorf("bad path: %w", err)
		}
	}

	out, err = manage(vboxmanage, "modifyvm", name, "--uartmode1", "client", consolePath)
	if err != nil {
		return fmt.Errorf("modifyvm --uartmode error: %w\n%s", err, out)
	}

	out, err = manage(vboxmanage, "storagectl", name, "--name", "IDE Controller", "--add", "ide")
	if err != nil {
		return fmt.Errorf("storagectl error: %w\n%s", err, out)
	}

	out, err = manage(vboxmanage, "storageattach", name, "--storagectl", "IDE Controller", "--port", "1", "--device", "0", "--type", "hdd", "--medium", path)
	if err != nil {
		return fmt.Errorf("storageattach error: %w\n%s", err, out)
	}
	out, err = manage(vboxmanage, "modifyvm", name, "--boot1", "disk")
	if err != nil {
		return fmt.Errorf("modifyvm --boot error: %w\n%s", err, out)
	}

	if len(disks) > 0 {
		out, err = manage(vboxmanage, "storagectl", name, "--name", "SATA", "--add", "sata")
		if err != nil {
			return fmt.Errorf("storagectl error: %w\n%s", err, out)
		}
	}

	for i, d := range disks {
		id := strconv.Itoa(i)
		if d.Size != 0 && d.Format == "" {
			d.Format = "raw"
		}
		if d.Format != "raw" && d.Path == "" {
			return fmt.Errorf("vbox currently can only create raw disks")
		}
		if d.Path == "" && d.Size == 0 {
			return fmt.Errorf("please specify an existing disk file or a size")
		}
		if d.Path == "" {
			d.Path = "disk" + id + ".img"
			if err := os.Truncate(d.Path, int64(d.Size)*int64(1048576)); err != nil {
				return fmt.Errorf("cannot create disk: %w", err)
			}
		}
		out, err = manage(vboxmanage, "storageattach", name, "--storagectl", "SATA", "--port", "0", "--device", id, "--type", "hdd", "--medium", d.Path)
		if err != nil {
			return fmt.Errorf("storageattach error: %w\n%s", err, out)
		}
	}

	for i, d := range networks {
		nic := i + 1
		out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--nictype%d", nic), "virtio")
		if err != nil {
			return fmt.Errorf("modifyvm --nictype error: %w\n%s", err, out)
		}

		out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--nic%d", nic), d.Type)
		if err != nil {
			return fmt.Errorf("modifyvm --nic error: %w\n%s", err, out)
		}
		switch d.Type {
		case "hostonly":
			out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--hostonlyadapter%d", nic), d.Adapter)
			if err != nil {
				return fmt.Errorf("modifyvm --hostonlyadapter error: %w\n%s", err, out)
			}
		case "bridged":
			out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--bridgeadapter%d", nic), d.Adapter)
			if err != nil {
				return fmt.Errorf("modifyvm --bridgeadapter error: %w\n%s", err, out)
			}
		}

		out, err = manage(vboxmanage, "modifyvm", name, fmt.Sprintf("--cableconnected%d", nic), "on")
		if err != nil {
			return fmt.Errorf("modifyvm --cableconnected error: %w\n%s", err, out)
		}
	}

	// create socket
	_ = os.Remove(consolePath)
	ln, err := net.Listen("unix", consolePath)
	if err != nil {
		return fmt.Errorf("cannot listen on console socket %s: %w", consolePath, err)
	}
	defer ln.Close()

	var vmType string
	if enableGUI {
		vmType = "gui"
	} else {
		vmType = "headless"
	}

	term := console.Current()
	ws, err := term.Size()
	if err != nil {
		return fmt.Errorf("get term size: %w", err)
	}

	out, err = manage(vboxmanage, "startvm", name, "--type", vmType)
	if err != nil {
		return fmt.Errorf("startvm error: %w\n%s", err, out)
	}
	defer cleanup(vboxmanage, name)

	if err := term.Resize(ws); err != nil && !errors.Is(err, console.ErrUnsupported) {
		return fmt.Errorf("resize term: %w", err)
	}
	if err := term.SetRaw(); err != nil {
		return fmt.Errorf("set raw term: %w", err)
	}
	defer func() {
		if err := term.Reset(); err != nil {
			logrus.Errorf("failed to reset term: %v", err)
		}
	}()

	socket, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept error: %w", err)
	}
	defer socket.Close()
	errs := make(chan error, 2)
	go func() {
		_, err := io.Copy(socket, term)
		errs <- err
	}()
	go func() {
		_, err := io.Copy(term, socket)
		errs <- err
	}()
	return <-errs
}

func cleanup(vboxmanage string, name string, logErrs ...bool) {
	logErr := true
	if len(logErrs) > 0 {
		logErr = logErrs[0]
	}
	if _, err := manage(vboxmanage, "controlvm", name, "poweroff"); err != nil && logErr {
		logrus.Errorf("controlvm poweroff error: %v", err)
	}
	out, err := manage(vboxmanage, "storageattach", name, "--storagectl", "IDE Controller", "--port", "1", "--device", "0", "--type", "hdd", "--medium", "emptydrive")
	if err != nil && logErr {
		logrus.Errorf("storageattach error: %v\n%s", err, out)
	}
	for i := range disks {
		id := strconv.Itoa(i)
		out, err := manage(vboxmanage, "storageattach", name, "--storagectl", "SATA", "--port", "0", "--device", id, "--type", "hdd", "--medium", "emptydrive")
		if err != nil && logErr {
			logrus.Errorf("storageattach error: %v\n%s", err, out)
		}
	}
	if out, err = manage(vboxmanage, "unregistervm", name, "--delete"); err != nil && logErr {
		logrus.Errorf("unregistervm error: %v\n%s", err, out)
	}
}

func manage(vboxmanage string, args ...string) (string, error) {
	logrus.Debugf("$ %s %s", vboxmanage, strings.Join(args, " "))
	cmd := exec.Command(vboxmanage, args...)

	var stderr bytes.Buffer
	cmd.Stdout = logrus.StandardLogger().WriterLevel(logrus.DebugLevel)
	cmd.Stderr = io.MultiWriter(&stderr, logrus.StandardLogger().WriterLevel(logrus.DebugLevel))
	err := cmd.Run()
	return stderr.String(), err
}

// VBNetwork is the config for a Virtual Box network
type VBNetwork struct {
	Type    string
	Adapter string
}

// VBNetworks is the type for a list of VBNetwork
type VBNetworks []VBNetwork

func (l *VBNetworks) String() string {
	return fmt.Sprint(*l)
}

func (l *VBNetworks) Type() string {
	return "vbnetworks"
}

// Set is used by flag to configure value from CLI
func (l *VBNetworks) Set(value string) error {
	d := VBNetwork{}
	s := strings.SplitSeq(value, ",")
	for p := range s {
		c := strings.SplitN(p, "=", 2)
		switch len(c) {
		case 1:
			d.Type = c[0]
		case 2:
			switch c[0] {
			case "type":
				d.Type = c[1]
			case "adapter", "bridgeadapter", "hostadapter":
				d.Adapter = c[1]
			default:
				return fmt.Errorf("unknown network config: %s", c[0])
			}
		}
	}
	*l = append(*l, d)
	return nil
}
