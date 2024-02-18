// Golang port of Overleaf
// Copyright (C) 2022-2024 Jakob Ackermann <das7pad@outlook.com>
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

package sharedTypes

import (
	"testing"
)

func TestImageName_Validate(t *testing.T) {
	tests := []struct {
		name    string
		i       ImageName
		wantErr bool
	}{
		{
			name:    "missing",
			i:       "",
			wantErr: true,
		},
		{
			name:    "no year in tag",
			i:       "texlive:foo",
			wantErr: true,
		},
		{
			name:    "ol",
			i:       "texlive:2022.1",
			wantErr: false,
		},
		{
			name:    "just year",
			i:       "texlive:2022",
			wantErr: false,
		},
		{
			name:    "TL2021-historic",
			i:       "texlive/texlive:TL2021-historic",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.i.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
