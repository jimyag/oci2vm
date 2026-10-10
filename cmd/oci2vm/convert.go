// Copyright 2022 Linka Cloud  All rights reserved.
// Modifications copyright 2026 jimyag. See the git history for details.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/c2h5oh/datasize"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jimyag/oci2vm"
	"github.com/jimyag/oci2vm/pkg/docker"
)

var convertCmd = &cobra.Command{
	Use:          "convert [docker image]",
	Short:        "Convert Docker image to vm image",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" || !isRoot() {
			abs, err := filepath.Abs(output)
			if err != nil {
				return err
			}
			out := filepath.Dir(abs)
			dargs := os.Args[2:]
			for i, v := range dargs {
				if v == output {
					dargs[i] = filepath.Join("/out", filepath.Base(output))
					break
				}
			}
			return docker.RunOCI2VM(cmd.Context(), oci2vm.Image, oci2vm.Version, out, out, cmd.Name(), dargs...)
		}
		if err := validateFlags(); err != nil {
			return err
		}
		size, err := parseSize(size)
		if err != nil {
			return err
		}
		img := args[0]
		found := false
		if !pull {
			imgs, err := docker.ImageList(cmd.Context(), img)
			if err != nil {
				return err
			}
			found = len(imgs) == 1 && imgs[0] == img
			if found {
				logrus.Infof("using local image %s", img)
			}
		}
		if pull || !found {
			logrus.Infof("pulling image %s", img)
			if err := docker.Pull(cmd.Context(), platform, img); err != nil {
				return err
			}
		}
		if err := oci2vm.Convert(
			cmd.Context(),
			img,
			oci2vm.WithSize(size),
			oci2vm.WithPassword(password),
			oci2vm.WithOutput(output),
			oci2vm.WithCmdLineExtra(cmdLineExtra),
			oci2vm.WithNetworkManager(oci2vm.NetworkManager(networkManager)),
			oci2vm.WithBootLoader(bootloader),
			oci2vm.WithRaw(raw),
			oci2vm.WithSplitBoot(splitBoot),
			oci2vm.WithBootSize(bootSize),
			oci2vm.WithBootFS(oci2vm.BootFS(bootFS)),
			oci2vm.WithLuksPassword(luksPassword),
			oci2vm.WithKeepCache(keepCache),
			oci2vm.WithPlatform(platform),
			oci2vm.WithPull(pull),
			oci2vm.WithHostname(hostname),
			oci2vm.WithDNS(dns),
			oci2vm.WithDNSSearch(dnsSearch),
			oci2vm.WithExtraHosts(extraHosts),
		); err != nil {
			return err
		}
		if err := chownOutput(output); err != nil {
			return err
		}
		return maybeMakeContainerDisk(cmd.Context())
	},
}

func parseSize(s string) (uint64, error) {
	var v datasize.ByteSize
	if err := v.UnmarshalText([]byte(s)); err != nil {
		return 0, err
	}
	return uint64(v), nil
}

func init() {
	convertCmd.Flags().AddFlagSet(buildFlags())
	rootCmd.AddCommand(convertCmd)
}
