// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type EditorEventMessage string

const (
	Bootstrap                       = EditorEventMessage("bootstrap")
	BroadcastDocMeta                = EditorEventMessage("broadcastDocMeta")
	ClientTrackingBatch             = EditorEventMessage("clientTracking.batch")
	ClientTrackingUpdated           = EditorEventMessage("clientTracking.clientUpdated")
	CompilerUpdated                 = EditorEventMessage("compilerUpdated")
	ConnectionRejected              = EditorEventMessage("connectionRejected")
	ForceDisconnect                 = EditorEventMessage("forceDisconnect")
	ImageNameUpdated                = EditorEventMessage("imageNameUpdated")
	NewChatMessage                  = EditorEventMessage("new-chat-message")
	OtUpdateApplied                 = EditorEventMessage("otUpdateApplied")
	OtUpdateError                   = EditorEventMessage("otUpdateError")
	ProjectMembershipChanged        = EditorEventMessage("project:membership:changed")
	ProjectNameUpdated              = EditorEventMessage("projectNameUpdated")
	ProjectPublicAccessLevelChanged = EditorEventMessage("project:publicAccessLevel:changed")
	ReceiveEntityMove               = EditorEventMessage("receiveEntityMove")
	ReceiveEntityRename             = EditorEventMessage("receiveEntityRename")
	ReceiveNewDoc                   = EditorEventMessage("receiveNewDoc")
	ReceiveNewFile                  = EditorEventMessage("receiveNewFile")
	ReceiveNewFolder                = EditorEventMessage("receiveNewFolder")
	RemoveEntity                    = EditorEventMessage("removeEntity")
	RootDocUpdated                  = EditorEventMessage("rootDocUpdated")
	SpellCheckLanguageUpdated       = EditorEventMessage("spellCheckLanguageUpdated")
)

var errBadEditorEventMessage = errors.New("bad editor event message")

func (e *EditorEventMessage) UnmarshalJSON(p []byte) error {
	if len(p) < 2 || p[0] != '"' || p[len(p)-1] != '"' {
		return errBadEditorEventMessage
	}
	switch EditorEventMessage(p[1 : len(p)-1]) {
	case Bootstrap:
		*e = Bootstrap
	case BroadcastDocMeta:
		*e = BroadcastDocMeta
	case ClientTrackingBatch:
		*e = ClientTrackingBatch
	case ClientTrackingUpdated:
		*e = ClientTrackingUpdated
	case ConnectionRejected:
		*e = ConnectionRejected
	case CompilerUpdated:
		*e = CompilerUpdated
	case ForceDisconnect:
		*e = ForceDisconnect
	case ImageNameUpdated:
		*e = ImageNameUpdated
	case NewChatMessage:
		*e = NewChatMessage
	case OtUpdateApplied:
		*e = OtUpdateApplied
	case OtUpdateError:
		*e = OtUpdateError
	case ProjectMembershipChanged:
		*e = ProjectMembershipChanged
	case ProjectNameUpdated:
		*e = ProjectNameUpdated
	case ReceiveEntityMove:
		*e = ReceiveEntityMove
	case ReceiveEntityRename:
		*e = ReceiveEntityRename
	case ReceiveNewDoc:
		*e = ReceiveNewDoc
	case ReceiveNewFile:
		*e = ReceiveNewFile
	case ReceiveNewFolder:
		*e = ReceiveNewFolder
	case RemoveEntity:
		*e = RemoveEntity
	case RootDocUpdated:
		*e = RootDocUpdated
	case SpellCheckLanguageUpdated:
		*e = SpellCheckLanguageUpdated
	default:
		return errBadEditorEventMessage
	}
	return nil
}

type EditorEvent struct {
	/* "h" is a virtual field indicating the length of Payload */
	Payload     json.RawMessage    `json:"payload"`
	RoomId      UUID               `json:"room_id"`
	Message     EditorEventMessage `json:"message"`
	ProcessedBy string             `json:"processedBy,omitempty"`
	Source      PublicId           `json:"source,omitempty"`
}

func (e *EditorEvent) MarshalJSON() ([]byte, error) {
	o := make([]byte, 0, 20+len(`{"h":,"payload":,"room_id":"00000000-0000-0000-0000-000000000000","message":,"processed_by":,"source":}`)+len(e.Message)+len(e.Payload)+len(e.ProcessedBy)+len(e.Source))
	o = append(o, `{"h":`...)
	o = strconv.AppendUint(o, uint64(len(e.Payload)), 10)
	o = append(o, `,"payload":`...)
	o = append(o, e.Payload...)
	o = append(o, `,"room_id":"`...)
	o = e.RoomId.Append(o)
	o = append(o, `","message":"`...)
	o = append(o, e.Message...)
	if len(e.ProcessedBy) > 0 {
		o = append(o, `","processedBy":"`...)
		o = append(o, e.ProcessedBy...)
	}
	if len(e.Source) > 0 {
		o = append(o, `","source":"`...)
		o = append(o, e.Source...)
	}
	o = append(o, `"}`...)
	return o, nil
}

func (e *EditorEvent) Validate() error {
	if e.Message == "" {
		return &errors.ValidationError{Msg: "missing message"}
	}
	return nil
}

var errMissingBodyHint = errors.New("missing body hint")

var (
	editorEventPayloadHint = []byte(`{"h":`)
	editorEventPayload     = []byte(`"payload":`)
)

func (e *EditorEvent) parseBodyHint(p []byte) (int, int) {
	if !bytes.HasPrefix(p, editorEventPayloadHint) || len(p) < 7 {
		return -1, 0
	}
	if p[5] == '0' && p[6] == ',' {
		return 7, 0
	}
	idx := bytes.IndexByte(p[5:], ',')
	if idx == -1 || idx == 0 || !bytes.HasPrefix(p[5+idx+1:], editorEventPayload) {
		return -1, 0
	}
	idx += 5
	v, err := strconv.ParseUint(string(p[5:idx]), 10, 64)
	if err != nil {
		return -1, 0
	}
	h := int(v)
	idx += 1 + len(editorEventPayload)
	if len(p) < idx+h+1 {
		return -1, 0
	}
	return idx, h
}

func (e *EditorEvent) FastUnmarshalJSON(p []byte) error {
	i, bodyHint := e.parseBodyHint(p)
	if i == -1 {
		return errMissingBodyHint
	}
	if bodyHint > 0 {
		j := i + bodyHint
		e.Payload = p[i:j]
		i = j + 1
	}
	p[i-1] = '{'
	return json.Unmarshal(p[i-1:], e)
}
