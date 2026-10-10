// Copyright 2022 Linka Cloud  All rights reserved.
// Modifications copyright 2026 jimyag. See the git history for details.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/spf13/cobra"

	"github.com/jimyag/oci2vm/cmd/oci2vm/run"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the virtual machine image",
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.AddCommand(run.VboxCmd)
	runCmd.AddCommand(run.QemuCmd)
	runCmd.AddCommand(run.HetznerCmd)
}
