// Copyright 2026 jimyag
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
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/sirupsen/logrus"
)

// chownOutput gives the generated file to the user who ran d2vm with sudo or
// through the docker wrapper (SUDO_UID), or else to the owner of the output
// directory, e.g. the host directory mounted by "docker run -v $PWD:/d2vm".
func chownOutput(path string) error {
	if !isRoot() {
		return nil
	}
	uid, gid, ok := sudoUser()
	if !ok {
		fi, err := os.Stat(filepath.Dir(path))
		if err != nil {
			return err
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || st.Uid == 0 {
			return nil
		}
		uid, gid = int(st.Uid), int(st.Gid)
	}
	return os.Chown(path, uid, gid)
}

func sudoUser() (uid, gid int, ok bool) {
	v := os.Getenv("SUDO_UID")
	if v == "" {
		return 0, 0, false
	}
	uid, err := strconv.Atoi(v)
	if err != nil {
		logrus.Errorf("invalid SUDO_UID: %s", v)
		return 0, 0, false
	}
	// the docker wrapper of older d2vm versions only sets SUDO_UID
	gid, err = strconv.Atoi(os.Getenv("SUDO_GID"))
	if err != nil {
		gid = uid
	}
	return uid, gid, true
}
