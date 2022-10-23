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

package httpUtils

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
)

func RespondPlain(c *Context, status int, body string) {
	EndTotalTimer(c)
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.Header().Set(
		"Content-Length", strconv.FormatInt(int64(len(body)), 10),
	)
	c.Writer.WriteHeader(status)
	_, _ = c.Writer.Write([]byte(body))
}

func RespondErr(c *Context, err error) {
	Respond(c, 0, nil, err)
}

func Respond(
	c *Context,
	code int,
	body interface{},
	err error,
) {
	respondJSON(c, code, body, err, false)
}

func RespondWithIndent(
	c *Context,
	code int,
	body interface{},
	err error,
) {
	respondJSON(c, code, body, err, true)
}

var fatalSerializeError []byte

func respondJSON(
	c *Context,
	code int,
	body interface{},
	err error,
	indent bool,
) {
	if err != nil {
		var errMessage string
		code, errMessage = GetAndLogErrResponseDetails(c, err)
		if body == nil {
			body = map[string]string{"message": errMessage}
		} else if r, ok := body.(*asyncForm.Response); ok && r.Message == nil {
			r.Message = &asyncForm.Message{
				Text: errMessage,
				Type: asyncForm.Error,
			}
		}
		if code == http.StatusTooManyRequests {
			if e, ok := errors.GetCause(err).(*errors.RateLimitedError); ok {
				s := int64(math.Ceil(e.RetryIn.Seconds()))
				c.Writer.Header().Set(
					"Retry-After", strconv.FormatInt(s, 10),
				)
			}
		}
	}
	EndTotalTimer(c)
	if body == nil {
		if code != http.StatusNoContent {
			c.Writer.Header().Set("Content-Length", "0")
		}
		c.Writer.WriteHeader(code)
		return
	}
	var blob []byte
	if indent {
		blob, err = json.MarshalIndent(body, "", "  ")
	} else {
		blob, err = json.Marshal(body)
	}
	if err != nil {
		GetAndLogErrResponseDetails(
			c, errors.Tag(err, "cannot serialize body"),
		)
		code = http.StatusInternalServerError
		blob = fatalSerializeError
	}
	c.Writer.Header().Set(
		"Content-Length", strconv.FormatInt(int64(len(blob)), 10),
	)
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(code)
	_, _ = c.Writer.Write(blob)
}

func GetAndLogErrResponseDetails(c *Context, err error) (int, string) {
	code := 500
	errMessage := err.Error()
	switch {
	case errors.IsValidationError(err):
		code = http.StatusBadRequest
	case errors.IsUnauthorizedError(err):
		code = http.StatusUnauthorized
	case errors.IsNotAuthorizedError(err):
		code = http.StatusForbidden
	case errors.IsDocNotFoundError(err):
		code = http.StatusNotFound
	case errors.IsMissingOutputFileError(err):
		code = http.StatusNotFound
	case errors.IsNotFoundError(err):
		code = http.StatusNotFound
	case errors.IsInvalidState(err):
		code = http.StatusConflict
	case errors.IsBodyTooLargeError(err):
		code = http.StatusRequestEntityTooLarge
	case errors.IsUnprocessableEntity(err):
		code = http.StatusUnprocessableEntity
	case errors.IsUpdateRangeNotAvailableError(err):
		code = http.StatusUnprocessableEntity
	case errors.IsAlreadyCompiling(err):
		code = http.StatusLocked
	case errors.IsRateLimitedError(err):
		code = http.StatusTooManyRequests
	default:
		log.Printf(
			"%s %s: %s",
			c.Request.Method, c.Request.URL.Path, errMessage,
		)
		code = http.StatusInternalServerError
		errMessage = "internal server error"
	}
	return code, errMessage
}

func init() {
	var err error
	fatalSerializeError, err = json.Marshal(
		map[string]string{"message": "internal server error"},
	)
	if err != nil {
		panic(errors.Tag(err, "cannot build fatalSerializeError"))
	}
}
