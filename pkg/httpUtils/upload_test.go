// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

package httpUtils

import (
	"testing"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Test_parseFilenameFromCD(t *testing.T) {
	tests := []struct {
		name    string
		cd      string
		want    sharedTypes.Filename
		wantErr bool
	}{
		{
			name:    "simple",
			cd:      `attachment; filename="foo"`,
			want:    "foo",
			wantErr: false,
		},
		{
			name:    "utf-8",
			cd:      `attachment; filename*=UTF-8''foo%2D%C3%A4%2D%E2%82%AC`,
			want:    "foo-ä-€",
			wantErr: false,
		},
		{
			name:    "empty header",
			cd:      "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty filename",
			cd:      `attachment; filename=`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "path",
			cd:      `attachment; filename="foo/bar.tex"`,
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFilenameFromCD(tt.cd)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFilenameFromCD() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseFilenameFromCD() got = %v, want %v", got, tt.want)
			}
		})
	}
}
