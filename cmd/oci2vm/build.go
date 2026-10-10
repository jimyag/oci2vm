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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jimyag/oci2vm"
	"github.com/jimyag/oci2vm/pkg/docker"
)

var (
	file      = "Dockerfile"
	tag       = "oci2vm-" + uuid.New().String()
	buildArgs []string
	buildCmd  = &cobra.Command{
		Use:   "build [context directory]",
		Short: "Build a vm image from Dockerfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(adphi): resolve context path
			if runtime.GOOS != "linux" || !isRoot() {
				ctxAbsPath, err := filepath.Abs(args[0])
				if err != nil {
					return err
				}
				if file == "" {
					file = filepath.Join(args[0], "Dockerfile")
				}
				dockerFileAbsPath, err := filepath.Abs(file)
				if err != nil {
					return err
				}
				if !strings.HasPrefix(dockerFileAbsPath, ctxAbsPath) {
					return fmt.Errorf("Dockerfile must be in the context directory path")
				}
				outputPath, err := filepath.Abs(output)
				if err != nil {
					return err
				}
				var (
					in  = ctxAbsPath
					out = filepath.Dir(outputPath)
				)
				dargs := os.Args[2:]
				for i, v := range dargs {
					switch v {
					case file:
						rel, err := filepath.Rel(in, dockerFileAbsPath)
						if err != nil {
							return fmt.Errorf("failed to construct Dockerfile container paths: %w", err)
						}
						dargs[i] = filepath.Join("/in", rel)
					case output:
						dargs[i] = filepath.Join("/out", filepath.Base(output))
					case args[0]:
						dargs[i] = "/in"
					}
				}
				return docker.RunOCI2VM(cmd.Context(), oci2vm.Image, oci2vm.Version, in, out, cmd.Name(), os.Args[2:]...)
			}
			if err := validateFlags(); err != nil {
				return err
			}
			size, err := parseSize(size)
			if err != nil {
				return err
			}
			if file == "" {
				file = filepath.Join(args[0], "Dockerfile")
			}
			logrus.Infof("building docker image from %s", file)
			if err := docker.Build(cmd.Context(), pull, tag, file, args[0], platform, buildArgs...); err != nil {
				return err
			}
			// the image is only built to be converted
			if !keepCache {
				defer docker.Remove(cmd.Context(), tag)
			}
			if err := oci2vm.Convert(
				cmd.Context(),
				tag,
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
				oci2vm.WithPull(false),
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
)

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().StringVarP(&file, "file", "f", "", "Name of the Dockerfile")
	buildCmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "Set build-time variables")

	buildCmd.Flags().AddFlagSet(buildFlags())
}
