package oci2vm

import (
	"bytes"
	"strings"
	"testing"
)

func TestPackageManagerBootstrapDockerfile(t *testing.T) {
	for _, tc := range []struct {
		release OSRelease
		base    string
		install string
	}{
		{OSRelease{ID: ReleaseUbuntu, VersionID: "26.04"}, "ubuntu:26.04", "dpkg -i /tmp/oci2vm-packages/*.deb"},
		{OSRelease{ID: ReleaseCentOS, VersionID: "9"}, "quay.io/centos/centos:stream9", "COPY --from=original /etc/ /etc/"},
		{OSRelease{ID: ReleaseDebian, VersionID: "13"}, "debian:13", "dpkg -i /tmp/oci2vm-packages/*.deb"},
		{OSRelease{ID: ReleaseKali, VersionCodeName: "kali-rolling"}, "kalilinux/kali-rolling:latest", "dpkg -i /tmp/oci2vm-packages/*.deb"},
		{OSRelease{ID: ReleaseAlpine, VersionID: "3.22.1"}, "alpine:3.22", "/tmp/oci2vm-apk fix --no-cache --upgrade apk-tools && apk --version"},
		{OSRelease{ID: ReleaseRocky, VersionID: "8.10"}, "rockylinux:8", "--releasever=8"},
		{OSRelease{ID: ReleaseAlmaLinux, VersionID: "10.0"}, "almalinux:10", "--releasever=10"},
	} {
		t.Run(string(tc.release.ID), func(t *testing.T) {
			d, err := NewDockerfile(tc.release, "component:original", "", "", false, false, true)
			if err != nil {
				t.Fatal(err)
			}
			var normal bytes.Buffer
			if err := d.Render(&normal); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(normal.String(), "AS packages") {
				t.Fatal("normal images unexpectedly restore a package manager")
			}
			d.BootstrapImage = tc.base
			var prepared bytes.Buffer
			if err := d.Render(&prepared); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"FROM " + tc.base + " AS packages", "FROM component:original", tc.install} {
				if !strings.Contains(prepared.String(), want) {
					t.Errorf("prepared Dockerfile missing %q", want)
				}
			}
		})
	}
}

func TestPackageManagerBootstrapImage(t *testing.T) {
	for _, tc := range []struct {
		release OSRelease
		want    string
	}{
		{OSRelease{ID: ReleaseUbuntu, VersionID: "24.04"}, "ubuntu:24.04"},
		{OSRelease{ID: ReleaseDebian, VersionID: "9"}, "debian:9"},
		{OSRelease{ID: ReleaseDebian, VersionID: "11"}, "debian:11"},
		{OSRelease{ID: ReleaseDebian, VersionID: "13"}, "debian:13"},
		{OSRelease{ID: ReleaseKali, VersionCodeName: "kali-rolling"}, "kalilinux/kali-rolling:latest"},
		{OSRelease{ID: ReleaseAlpine, VersionID: "3.22.1"}, "alpine:3.22"},
		{OSRelease{ID: ReleaseCentOS, VersionID: "7"}, "centos:7"},
		{OSRelease{ID: ReleaseCentOS, VersionID: "8"}, "centos:8"},
		{OSRelease{ID: ReleaseCentOS, VersionID: "8", Name: "CentOS Stream"}, "quay.io/centos/centos:stream8"},
		{OSRelease{ID: ReleaseCentOS, VersionID: "9", Name: "CentOS Stream"}, "quay.io/centos/centos:stream9"},
		{OSRelease{ID: ReleaseCentOS, VersionID: "10", Name: "CentOS Stream"}, "quay.io/centos/centos:stream10"},
		{OSRelease{ID: ReleaseRocky, VersionID: "8.10"}, "rockylinux:8"},
		{OSRelease{ID: ReleaseAlmaLinux, VersionID: "10.0"}, "almalinux:10"},
	} {
		got, err := packageManagerBootstrapImage(tc.release)
		if err != nil || got != tc.want {
			t.Errorf("%+v: got %q, %v; want %q", tc.release, got, err, tc.want)
		}
	}
	for _, release := range []OSRelease{
		{ID: ReleaseUbuntu, VersionID: "24.04\nRUN false"},
		{ID: ReleaseAlpine, VersionID: "3"},
		{ID: ReleaseCentOS, VersionID: "9", Name: "CentOS Linux"},
		{ID: ReleaseKali, VersionCodeName: "kali-last-snapshot"},
		{ID: ReleaseRHEL, VersionID: "9"},
	} {
		if _, err := packageManagerBootstrapImage(release); err == nil {
			t.Errorf("accepted unsupported restoration for %+v", release)
		}
	}
}
