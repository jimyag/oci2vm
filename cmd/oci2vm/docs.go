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
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate documentation",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		if err := os.MkdirAll(args[0], 0755); err != nil {
			logrus.Fatal(err)
		}
		cmd.Root().DisableAutoGenTag = true
		// the pages of the shell completion commands only repeat their help
		for _, c := range cmd.Root().Commands() {
			if c.Name() == "completion" {
				c.Hidden = true
			}
		}
		if err := doc.GenMarkdownTree(cmd.Root(), args[0]); err != nil {
			logrus.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
