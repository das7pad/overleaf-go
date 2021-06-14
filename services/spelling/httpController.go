// Golang port of the Overleaf spelling service
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

	jwtMiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/spelling/pkg/managers/spelling"
	"github.com/das7pad/spelling/pkg/types"
)

func newHttpController(cm spelling.Manager) httpController {
	return httpController{sm: cm}
}

type httpController struct {
	sm spelling.Manager
}

type CorsOptions struct {
	AllowedOrigins []string
	SiteUrl        string
}

func (h *httpController) GetRouter(
	corsOptions CorsOptions,
	jwtOptions jwtMiddleware.Options,
) http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/status", h.status)

	jwtRouter := router.PathPrefix("/jwt/spelling/v20200714").Subrouter()
	jwtRouter.Use(cors(corsOptions))
	jwtRouter.Use(noCache())
	jwtRouter.Use(jwtMiddleware.New(jwtOptions).Handler)
	jwtRouter.Use(validateAndSetId("userId"))
	jwtRouter.
		NewRoute().
		Methods(http.MethodOptions).
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	jwtRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/check").
		HandlerFunc(h.check)
	jwtRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/dict").
		HandlerFunc(h.getDictionary)
	jwtRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/learn").
		HandlerFunc(h.learn)
	jwtRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/unlearn").
		HandlerFunc(h.unlearn)

	prefixes := []string{"", "/v20200714"}
	for _, prefix := range prefixes {
		router.
			NewRoute().
			Methods(http.MethodPost).
			Path(prefix + "/check").
			HandlerFunc(h.check)

		userRouter := router.
			PathPrefix(prefix + "/user/{userId}").
			Subrouter()
		userRouter.Use(validateAndSetId("userId"))
		userRouter.
			NewRoute().
			Methods(http.MethodDelete).
			Path("").
			HandlerFunc(h.deleteDictionary)
		userRouter.
			NewRoute().
			Methods(http.MethodGet).
			Path("").
			HandlerFunc(h.getDictionary)
		userRouter.
			NewRoute().
			Methods(http.MethodPost).
			Path("/check").
			HandlerFunc(h.check)
		userRouter.
			NewRoute().
			Methods(http.MethodGet).
			Path("/dict").
			HandlerFunc(h.getDictionary)
		userRouter.
			NewRoute().
			Methods(http.MethodPost).
			Path("/learn").
			HandlerFunc(h.learn)
		userRouter.
			NewRoute().
			Methods(http.MethodPost).
			Path("/unlearn").
			HandlerFunc(h.unlearn)
	}
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
					"GET, OPTIONS, POST",
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

func validateAndSetId(name string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := primitive.ObjectIDFromHex(getRawIdFromRequest(r, name))
			if err != nil || id == primitive.NilObjectID {
				errorResponse(w, http.StatusBadRequest, "invalid "+name)
				return
			}
			ctx := context.WithValue(r.Context(), name, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func getRawIdFromRequest(r *http.Request, name string) string {
	rawId := getRawIdFromPath(r, name)
	if rawId != "" {
		return rawId
	}
	return getRawIdFromJwt(r, name)
}
func getRawIdFromPath(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}
func getRawIdFromJwt(r *http.Request, name string) string {
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

func getId(r *http.Request, name string) primitive.ObjectID {
	id := r.Context().Value(name)
	if id == nil {
		// The validation middleware should have blocked this request.
		log.Printf(
			"%s not validated on route %s %s",
			name, r.Method, r.URL.Path,
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
	_, _ = w.Write([]byte("spelling is alive (go)\n"))
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
		if _, is400 := err.(spelling.ValidationError); is400 {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("%s %s: %s: %v", r.Method, r.URL.Path, msg, err)
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

type checkRequestBody struct {
	Language string   `json:"language"`
	Words    []string `json:"words"`
}

type checkResponseBody struct {
	Misspellings []types.Misspelling `json:"misspellings"`
}

func (h *httpController) check(w http.ResponseWriter, r *http.Request) {
	var requestBody checkRequestBody
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request")
		return
	}
	misspellings, err := h.sm.CheckWords(
		r.Context(),
		requestBody.Language,
		requestBody.Words,
	)
	responseBody := checkResponseBody{Misspellings: misspellings}
	respond(w, r, http.StatusOK, responseBody, err, "cannot check")
}

func (h *httpController) deleteDictionary(w http.ResponseWriter, r *http.Request) {
	err := h.sm.DeleteDictionary(
		r.Context(),
		getId(r, "userId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot get dictionary")
}

func (h *httpController) getDictionary(w http.ResponseWriter, r *http.Request) {
	dictionary, err := h.sm.GetDictionary(
		r.Context(),
		getId(r, "userId"),
	)
	respond(w, r, http.StatusOK, dictionary, err, "cannot get dictionary")
}

type learnRequestBody struct {
	Word string `json:"word"`
}

func parseLearnBody(r *http.Request) (string, error) {
	var requestBody learnRequestBody
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	return requestBody.Word, err
}

func (h *httpController) learn(w http.ResponseWriter, r *http.Request) {
	word, err := parseLearnBody(r)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request")
		return
	}
	err = h.sm.LearnWord(
		r.Context(),
		getId(r, "userId"),
		word,
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot learn word")
}

func (h *httpController) unlearn(w http.ResponseWriter, r *http.Request) {
	word, err := parseLearnBody(r)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request")
		return
	}
	err = h.sm.UnlearnWord(
		r.Context(),
		getId(r, "userId"),
		word,
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot unlearn word")
}
