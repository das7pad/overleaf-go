// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package outputCache

import (
	"log"
	"os"
	"time"

	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

func cleanupExpired(dir types.OutputDir, keepBuildId types.BuildId, namespace types.Namespace) {
	dirs, scanErr := os.ReadDir(dir.CompileOutput())
	if scanErr != nil {
		if os.IsNotExist(scanErr) {
			return
		}
		log.Printf("scan outputDir %s for cleanup: %s", dir, scanErr)
		return
	}

	keepLastNBuilds := 2
	if namespace.IsAnonymous() {
		keepLastNBuilds = 3
	}

	n := len(dirs)
	// dirs is sorted by time ASC
	for i, entry := range dirs {
		buildId := types.BuildId(entry.Name())
		if buildId == keepBuildId {
			continue
		}

		if age, err := buildId.Age(); err != nil {
			log.Printf(
				"parse age from buildId %q in %s: %s",
				buildId, dir, err,
			)
			// Delete this directory unconditionally.
		} else {
			// Keep the last N builds of the past hour.
			isLastNth := n - i
			if isLastNth <= keepLastNBuilds && age < 1*time.Hour {
				break
			}
		}

		compileOutputDir := dir.CompileOutputDir(buildId)
		if err := os.RemoveAll(string(compileOutputDir)); err != nil {
			log.Printf("cleanup %s: %s", compileOutputDir, err)
		}
	}
}
