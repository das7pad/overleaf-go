// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

package seccomp

import (
	"embed"
	"encoding/json"
	"os"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

//goland:noinspection SpellCheckingInspection
type policy struct {
	DefaultAction string   `json:"defaultAction"`
	Architectures []string `json:"architectures"`
	SysCalls      []struct {
		Name   string `json:"name"`
		Action string `json:"action"`
		Args   []struct {
			Index    int    `json:"index"`
			Value    int    `json:"value"`
			ValueTwo int    `json:"valueTwo"`
			Op       string `json:"op"`
		} `json:"args"`
	} `json:"syscalls"`
}

//go:embed policy.json
var _bundled embed.FS

func Load(customPath string) (string, error) {
	var blob []byte
	var err error
	var source string
	if customPath != "" {
		source = "custom policy"
		blob, err = os.ReadFile(customPath)
	} else {
		source = "bundled policy"
		blob, err = _bundled.ReadFile("policy.json")
	}
	if err != nil {
		return "", errors.Tag(err, "read "+source)
	}
	var p policy
	if err = json.Unmarshal(blob, &p); err != nil {
		return "", errors.Tag(err, "deserialize "+source)
	}
	// Round trip the policy through the schema
	normalizedBlob, err := json.Marshal(p)
	if err != nil {
		return "", errors.Tag(err, "serialize "+source)
	}
	return string(normalizedBlob), nil
}
