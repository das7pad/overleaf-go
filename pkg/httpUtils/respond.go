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

package httpUtils

import (
	"context"
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

func Respond(c *Context, code int, body interface{}, err error) {
	respondJSON(c, code, body, err, false)
}

func RespondWithIndent(c *Context, code int, body interface{}, err error) {
	respondJSON(c, code, body, err, true)
}

var fatalSerializeError []byte

func respondJSON(c *Context, code int, body interface{}, err error, indent bool) {
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
	if c.Err() == context.Canceled &&
		errors.GetCause(err) == context.Canceled {
		// Unfortunately HTTP does not define a response status code for the
		//  case of an aborted request before the server produced a response.
		// The next best option is "Client Closed Request" from nginx.
		return 499, context.Canceled.Error()
	}

	code := http.StatusInternalServerError
	switch errors.GetCause(err).(type) {
	case *errors.ValidationError:
		code = http.StatusBadRequest
	case *errors.UnauthorizedError:
		code = http.StatusUnauthorized
	case *errors.NotAuthorizedError:
		code = http.StatusForbidden
	case *errors.DocNotFoundError:
		code = http.StatusNotFound
	case *errors.MissingOutputFileError:
		code = http.StatusNotFound
	case *errors.NotFoundError:
		code = http.StatusNotFound
	case *errors.InvalidStateError:
		code = http.StatusConflict
	case *errors.BodyTooLargeError:
		code = http.StatusRequestEntityTooLarge
	case *errors.UnprocessableEntityError:
		code = http.StatusUnprocessableEntity
	case *errors.UpdateRangeNotAvailableError:
		code = http.StatusUnprocessableEntity
	case *errors.AlreadyCompilingError:
		code = http.StatusLocked
	case *errors.RateLimitedError:
		code = http.StatusTooManyRequests
	default:
		log.Printf(
			"%s %s: %s",
			c.Request.Method, c.Request.URL.Path, err.Error(),
		)
		code = http.StatusInternalServerError
	}
	return code, errors.GetPublicMessage(err, "internal server error")
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
