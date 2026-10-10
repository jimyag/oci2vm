package oci2vm

import (
	"strings"
	"testing"
)

func TestBootSizeFromUsage(t *testing.T) {
	for _, tc := range []struct {
		usage string
		want  uint64
	}{
		{"0\t/boot\n", 128},
		{"65536\t/boot\n", 160},
		{"180224\t/boot\n", 256},
		{"524288\t/boot\n", 608},
	} {
		got, err := bootSizeFromUsage(tc.usage)
		if err != nil || got != tc.want {
			t.Errorf("bootSizeFromUsage(%q) = %d, %v; want %d", tc.usage, got, err, tc.want)
		}
	}
	for _, usage := range []string{"", "-1 /boot", "123 /root", "text /boot", "4294967296 /boot", "1 /boot\n2 /boot"} {
		if _, err := bootSizeFromUsage(usage); err == nil {
			t.Errorf("accepted invalid usage %q", usage)
		}
	}
}

func TestExplicitBootSizeValidation(t *testing.T) {
	for _, tc := range []struct {
		size uint64
		boot uint64
		want string
	}{
		{1024 * 1024 * 1024, 49, "at least 50MiB"},
		{100 * 1024 * 1024, 100, "less than the disk size"},
		{100 * 1024 * 1024, 256, "less than the disk size"},
	} {
		_, err := NewBuilder(t.Context(), t.TempDir(), "unused", "", tc.size, OSRelease{}, "qcow2", "", true, BootFSFat32, tc.boot, "", "grub-efi", "linux/arm64", "", nil, nil, nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("disk=%d boot=%d: got %v; want %q", tc.size, tc.boot, err, tc.want)
		}
	}
}
