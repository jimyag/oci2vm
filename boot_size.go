package oci2vm

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/jimyag/oci2vm/pkg/docker"
)

func inspectBootSize(ctx context.Context, image string) (uint64, error) {
	out, _, err := docker.CmdOut(ctx, "run", "--rm", "--user", "0", "--entrypoint", "du", image, "-sk", "/boot")
	if err != nil {
		return 0, fmt.Errorf("measure /boot in %s: %w", image, err)
	}
	size, err := bootSizeFromUsage(out)
	if err != nil {
		return 0, err
	}
	logrus.Infof("automatically sized boot partition: %d MiB", size)
	return size, nil
}

func bootSizeFromUsage(out string) (uint64, error) {
	fields := strings.Fields(out)
	if len(fields) != 2 || fields[1] != "/boot" {
		return 0, fmt.Errorf("unexpected /boot disk usage: %q", out)
	}
	kib, err := strconv.ParseUint(fields[0], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid /boot disk usage: %w", err)
	}
	// Reserve 64 MiB for the bootloader and filesystem, plus the 1 MiB start offset.
	mib := (kib+1023)/1024 + 65
	return max(uint64(128), (mib+31)/32*32), nil
}
