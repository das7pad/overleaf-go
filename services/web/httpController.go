// Golang port of the Overleaf web service
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

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	jwtMiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func newHttpController(wm web.Manager) httpController {
	return httpController{wm: wm}
}

type httpController struct {
	wm web.Manager
}

type CorsOptions struct {
	AllowedOrigins []string
	SiteUrl        string
}

type idField string

const (
	userIdField    idField = "userId"
	projectIdField idField = "projectId"
)

func (h *httpController) GetRouter(
	corsOptions CorsOptions,
	jwtOptions jwtMiddleware.Options,
	client redis.UniversalClient,
) http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/status", h.status)

	jwtRouter := router.PathPrefix("/jwt/web").Subrouter()
	jwtRouter.Use(cors(corsOptions))
	jwtRouter.Use(noCache())
	jwtRouter.Use(jwtMiddleware.New(jwtOptions).Handler)
	jwtRouter.Use(validateAndSetId(userIdField))
	jwtRouter.
		NewRoute().
		Methods(http.MethodOptions).
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})

	projectRouter := jwtRouter.
		PathPrefix("/project/{projectId}").
		Subrouter()
	projectRouter.Use(validateAndSetId(projectIdField))
	projectRouter.Use(checkEpochs(client))
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/clear-cache").
		HandlerFunc(h.clearProjectCache)
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/compile").
		HandlerFunc(h.compileProject)
	return router
}

func noCache() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache")
			next.ServeHTTP(w, r)
		})
	}
}

func checkEpochs(client redis.UniversalClient) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ids := r.Context().Value(validatedIds).([]idField)
			epochs := make(map[idField]*redis.StringCmd)
			_, err := client.Pipelined(r.Context(), func(p redis.Pipeliner) error {
				for _, field := range ids {
					epochs[field] = p.Get(
						r.Context(),
						"epoch:"+string(field)+":"+getId(r, field).Hex(),
					)
				}
				return nil
			})
			if err != nil {
				respond(w, r, 200, nil, err, "cannot validate epoch")
				return
			}
			for _, field := range ids {
				stored := epochs[field].Val()
				provided := getItemFromJwt(r, string("epoch_"+field))
				if stored != provided {
					errorResponse(w, 401, "epoch mismatch: "+string(field))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func cors(options CorsOptions) mux.MiddlewareFunc {
	siteUrl, err := url.Parse(options.SiteUrl)
	if err != nil {
		panic(err)
	}
	publicHost := siteUrl.Host
	allowedOrigins := make(map[string]bool)
	for _, origin := range options.AllowedOrigins {
		allowedOrigins[origin] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != publicHost {
				w.Header().Add("Vary", "Origin")
				origin := r.Header.Get("Origin")
				if allowedOrigins[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Set(
					"Access-Control-Allow-Headers",
					"Authorization,Content-Type",
				)
				w.Header().Set(
					"Access-Control-Allow-Methods",
					"DELETE, GET, OPTIONS",
				)
				w.Header().Set("Access-Control-Max-Age", "3600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

const (
	validatedIds = "validatedIds"
)

func validateAndSetId(field idField) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := string(field)
			idFromPath := getPathParam(r, name)
			idFromJwt := getItemFromJwt(r, name)
			rawId := idFromPath
			if idFromJwt != "" {
				if idFromPath != "" && idFromPath != idFromJwt {
					errorResponse(
						w,
						http.StatusBadRequest,
						"jwt id mismatches path id: "+name,
					)
					return
				}
				rawId = idFromJwt
			}
			id, err := primitive.ObjectIDFromHex(rawId)
			if err != nil || id == primitive.NilObjectID {
				errorResponse(w, http.StatusBadRequest, "invalid "+name)
				return
			}
			var ids []idField
			if previous := r.Context().Value(validatedIds); previous != nil {
				ids = previous.([]idField)
			}
			ids = append(ids, field)
			ctx := context.WithValue(r.Context(), field, id)
			ctx = context.WithValue(ctx, validatedIds, ids)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func getPathParam(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}
func getItemFromJwt(r *http.Request, name string) string {
	user := r.Context().Value("user")
	if user == nil {
		return ""
	}
	token := user.(*jwt.Token)
	if token == nil {
		return ""
	}
	claims := token.Claims.(jwt.MapClaims)
	idFromJwt := claims[name]
	if idFromJwt == nil {
		return ""
	}
	return idFromJwt.(string)
}
func getSignedCompileProjectOptionsFromJwt(r *http.Request) (*types.SignedCompileProjectRequestOptions, error) {
	// Already validated further up in stack.
	user := r.Context().Value("user")
	token := user.(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)

	o := &types.SignedCompileProjectRequestOptions{
		ProjectId: getId(r, "projectId"),
		UserId:    getId(r, "userId"),
	}
	for field, raw := range claims {
		if raw == nil {
			continue
		}
		s, isString := raw.(string)
		if !isString || s == "" {
			continue
		}
		switch field {
		case "compileGroup":
			o.CompileGroup = clsiTypes.CompileGroup(s)
		case "timeout":
			d, err := time.ParseDuration(s)
			if err != nil {
				return nil, err
			}
			o.Timeout = clsiTypes.Timeout(d)
		}
	}
	return o, nil
}

func getId(r *http.Request, field idField) primitive.ObjectID {
	id := r.Context().Value(field)
	if id == nil {
		// The validation middleware should have blocked this request.
		log.Printf(
			"%s not validated on route %s %s",
			field, r.Method, r.URL.Path,
		)
		panic("broken id validation")
	}
	return id.(primitive.ObjectID)
}

func errorResponse(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)

	// Flush it and ignore any errors.
	_, _ = w.Write([]byte(message))
}

func (h *httpController) status(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("web is alive (go)\n"))
}

func respond(
	w http.ResponseWriter,
	r *http.Request,
	code int,
	body interface{},
	err error,
	msg string,
) {
	if err != nil {
		if errors.IsValidationError(err) {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.IsNotAuthorizedError(err) {
			errorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.IsNotFoundError(err) {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.IsInvalidState(err) {
			errorResponse(w, http.StatusConflict, err.Error())
			return
		}
		if errors.IsUpdateRangeNotAvailableError(err) {
			errorResponse(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		log.Printf("%s %s: %s: %s", r.Method, r.URL.Path, msg, err)
		errorResponse(w, http.StatusInternalServerError, msg)
		return
	}
	if body == nil {
		w.WriteHeader(code)
	} else {
		w.Header().Set(
			"Content-Type",
			"application/json; charset=utf-8",
		)
		if code != http.StatusOK {
			w.WriteHeader(code)
		}
		_ = json.NewEncoder(w).Encode(body)
	}
}

type clearProjectCacheRequestBody struct {
	types.ClsiServerId `json:"clsi_server_id"`
}

func (h *httpController) clearProjectCache(w http.ResponseWriter, r *http.Request) {
	so, err := getSignedCompileProjectOptionsFromJwt(r)
	if err != nil || so == nil {
		errorResponse(w, http.StatusBadRequest, "invalid options in jwt")
		return
	}

	request := &clearProjectCacheRequestBody{}
	if err = json.NewDecoder(r.Body).Decode(request); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	err = h.wm.ClearProjectCache(
		r.Context(),
		*so,
		request.ClsiServerId,
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot clear project cache")
}

func (h *httpController) compileProject(w http.ResponseWriter, r *http.Request) {
	so, err := getSignedCompileProjectOptionsFromJwt(r)
	if err != nil || so == nil {
		errorResponse(w, http.StatusBadRequest, "invalid options in jwt")
		return
	}

	request := &types.CompileProjectRequest{}
	if err = json.NewDecoder(r.Body).Decode(request); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	request.SignedCompileProjectRequestOptions = *so
	response := &types.CompileProjectResponse{}
	err = h.wm.CompileProject(
		r.Context(),
		request,
		response,
	)
	respond(w, r, 200, response, err, "cannot compile project")
}
