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

package editor

import (
	"context"
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/email/pkg/spamSafe"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) TransferProjectOwnership(ctx context.Context, request *types.TransferProjectOwnershipRequest) error {
	projectId := request.ProjectId
	newOwnerId := request.NewOwnerId
	previousOwnerId := request.PreviousOwnerId

	if previousOwnerId == newOwnerId {
		return &errors.ValidationError{
			Msg: "cannot transfer ownership to self",
		}
	}

	for i := 0; i < 10; i++ {
		p := &project.ForProjectOwnershipTransfer{}
		if err := m.pm.GetProject(ctx, projectId, p); err != nil {
			return errors.Tag(err, "cannot get project")
		}

		{
			d, _ := p.GetPrivilegeLevelAuthenticated(previousOwnerId)
			err := d.PrivilegeLevel.CheckIsAtLeast(sharedTypes.PrivilegeLevelOwner)
			if err != nil {
				return err
			}
		}
		{
			d, err := p.GetPrivilegeLevelAuthenticated(newOwnerId)
			if err != nil {
				return errUserIsNotAMember
			}
			if d.IsTokenMember {
				return errUserIsNotAMember
			}
		}

		users, err := m.um.GetUsersForBackFilling(
			ctx, user.UniqUserIds{previousOwnerId: true, newOwnerId: true},
		)
		if err != nil {
			return errors.Tag(err, "cannot get user details")
		}
		previousOwner, exists := users[previousOwnerId]
		if !exists {
			return &errors.InvalidStateError{
				Msg: "current user has been deleted",
			}
		}
		newOwner, exists := users[newOwnerId]
		if !exists {
			return &errors.InvalidStateError{
				Msg: "new owner has been deleted",
			}
		}

		// Clearing the epoch is OK to do at any time.
		// Clear it just ahead of changing access.
		err = projectJWT.ClearProjectField(ctx, m.client, projectId)
		if err != nil {
			return err
		}
		err = m.pm.TransferOwnership(ctx, p, newOwnerId)
		// Clear it immediately after potentially changing access.
		_ = projectJWT.ClearProjectField(ctx, m.client, projectId)
		if err != nil {
			if errors.GetCause(err) == project.ErrEpochIsNotStable {
				continue
			}
			return errors.Tag(err, "cannot transfer ownership")
		}

		go m.notifyEditorAboutAccessChanges(projectId, &refreshMembershipDetails{
			Members: true,
			Owner:   true,
		})

		projectURL := m.options.SiteURL.WithPath("/project/" + p.Id.Hex())
		details := &transferOwnershipEmailDetails{
			previousOwner: previousOwner,
			newOwner:      newOwner,
			project:       p,
			projectURL:    &projectURL,
			emailOptions:  m.options.EmailOptions(),
		}
		previousOwnerErr := m.ownershipTransferConfirmationPreviousOwner(
			ctx, details,
		)
		newOwnerErr := m.ownershipTransferConfirmationNewOwner(
			ctx, details,
		)
		return errors.Merge(previousOwnerErr, newOwnerErr)
	}
	return project.ErrEpochIsNotStable
}

type transferOwnershipEmailDetails struct {
	previousOwner *user.WithPublicInfo
	newOwner      *user.WithPublicInfo
	project       *project.ForProjectOwnershipTransfer
	projectURL    *sharedTypes.URL
	emailOptions  *types.EmailOptions
}

func spamSafeUser(u *user.WithPublicInfo, placeholder string) string {
	name := u.DisplayName()
	if name != string(u.Email) && spamSafe.IsSafeUserName(name) {
		if spamSafe.IsSafeEmail(u.Email) {
			return name + " (" + string(u.Email) + ")"
		}
		return name
	}
	return spamSafe.GetSafeEmail(u.Email, placeholder)
}

func (m *manager) ownershipTransferConfirmationNewOwner(ctx context.Context, d *transferOwnershipEmailDetails) error {
	msg := fmt.Sprintf(
		"%s has made you the owner of %s. You can now manage its sharing settings.",
		spamSafeUser(d.previousOwner, "A collaborator"),
		spamSafe.GetSafeProjectName(d.project.Name, "a project"),
	)

	e := email.Email{
		Content: &email.CTAContent{
			PublicOptions: d.emailOptions.Public,
			Message:       email.Message{msg},
			Title: fmt.Sprintf(
				"%s - Owner change",
				spamSafe.GetSafeProjectName(d.project.Name, "A project"),
			),
			CTAText: "View project",
			CTAURL:  d.projectURL.String(),
		},
		Subject: "Project ownership transfer - " + m.options.AppName,
		To: &email.Identity{
			Address: d.newOwner.Email,
			DisplayName: spamSafe.GetSafeUserName(
				d.newOwner.DisplayName(), "",
			),
		},
	}
	if err := e.Send(ctx, d.emailOptions.Send); err != nil {
		return errors.Tag(err, "cannot email new owner")
	}
	return nil
}

func (m *manager) ownershipTransferConfirmationPreviousOwner(ctx context.Context, d *transferOwnershipEmailDetails) error {
	msg1 := fmt.Sprintf(
		"As per your request, we have made %s the owner of %s.",
		spamSafeUser(d.newOwner, "a collaborator"),
		spamSafe.GetSafeProjectName(d.project.Name, "your project"),
	)
	msg2 := fmt.Sprintf(
		"If you haven't asked to change the owner of %s, please get in touch with us via %s.",
		spamSafe.GetSafeProjectName(d.project.Name, "your project"),
		m.options.AdminEmail,
	)

	e := email.Email{
		Content: &email.CTAContent{
			PublicOptions: d.emailOptions.Public,
			Message:       email.Message{msg1, msg2},
			Title: fmt.Sprintf(
				"%s - Owner change",
				spamSafe.GetSafeProjectName(d.project.Name, "A project"),
			),
			CTAURL:  d.projectURL.String(),
			CTAText: "View project",
		},
		Subject: "Project ownership transfer - " + m.options.AppName,
		To: &email.Identity{
			Address: d.previousOwner.Email,
			DisplayName: spamSafe.GetSafeUserName(
				d.previousOwner.DisplayName(), "",
			),
		},
	}
	if err := e.Send(ctx, d.emailOptions.Send); err != nil {
		return errors.Tag(err, "cannot email previous owner")
	}
	return nil
}
