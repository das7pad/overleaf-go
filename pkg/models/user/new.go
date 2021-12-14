// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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

package user

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func NewUser(email sharedTypes.Email, hashedPW string) *ForCreation {
	now := time.Now().UTC()
	return &ForCreation{
		ForSession: ForSession{
			EpochField: EpochField{
				Epoch: 1,
			},
			IsAdminField: IsAdminField{
				IsAdmin: false,
			},
			ReferralIdField: ReferralIdField{
				ReferralId: "",
			},
			WithPublicInfo: WithPublicInfo{
				EmailField: EmailField{
					Email: email,
				},
				FirstNameField: FirstNameField{
					FirstName: "",
				},
				IdField: IdField{
					Id: primitive.NewObjectID(),
				},
				LastNameField: LastNameField{
					LastName: "",
				},
			},
		},
		AuditLogField: AuditLogField{
			AuditLog: make([]AuditLogEntry, 0),
		},
		EditorConfigField: EditorConfigField{
			EditorConfig: EditorConfig{
				AutoComplete:       true,
				AutoPairDelimiters: true,
				FontFamily:         editorFontFamilyLucida,
				FontSize:           12,
				LineHeight:         editorLineHightNormal,
				Mode:               editorModeDefault,
				OverallTheme:       editorOverallThemeNone,
				PDFViewer:          editorPdfViewerPdfjs,
				SyntaxValidation:   false,
				SpellCheckLanguage: "en",
				Theme:              "textmate",
			},
		},
		EmailsField: EmailsField{
			Emails: []EmailDetails{
				{
					Id:               primitive.NewObjectID(),
					CreatedAt:        now,
					Email:            email,
					ReversedHostname: email.ReversedHostname(),
				},
			},
		},
		FeaturesField: FeaturesField{
			Features: Features{
				Collaborators:       -1,
				Versioning:          true,
				CompileTimeout:      180,
				CompileGroup:        "standard",
				TrackChanges:        true,
				TrackChangesVisible: true,
			},
		},
		HashedPasswordField: HashedPasswordField{
			HashedPassword: hashedPW,
		},
		LoginCountField: LoginCountField{
			LoginCount: 0,
		},
		SignUpDateField: SignUpDateField{
			SignUpDate: now,
		},
	}
}
