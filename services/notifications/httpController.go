// Golang port of the Overleaf notifications service
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

	"github.com/form3tech-oss/jwt-go"

	jwtMiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/notifications/pkg/managers/notifications"
)

func newHttpController(cm notifications.Manager) httpController {
	return httpController{nm: cm}
}

type httpController struct {
	nm notifications.Manager
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

	jwtRouter := router.PathPrefix("/jwt/notifications").Subrouter()
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
		Methods(http.MethodGet).
		Path("").
		HandlerFunc(h.getNotifications)
	jwtNotificationRouter := jwtRouter.
		PathPrefix("/{notificationId}").
		Subrouter()
	jwtNotificationRouter.Use(validateAndSetId("notificationId"))
	jwtNotificationRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.removeNotificationById)

	userRouter := router.
		PathPrefix("/user/{userId}").
		Subrouter()
	userRouter.Use(validateAndSetId("userId"))
	userRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("").
		HandlerFunc(h.getNotifications)
	userRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("").
		HandlerFunc(h.addNotification)
	userRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.removeNotificationByKey)

	userNotificationRouter := userRouter.
		PathPrefix("/notification/{notificationId}").
		Subrouter()
	userNotificationRouter.Use(validateAndSetId("notificationId"))
	userNotificationRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.removeNotificationById)

	router.
		NewRoute().
		Methods(http.MethodDelete).
		Path("/key/{notificationKey}").
		HandlerFunc(h.removeNotificationByKeyOnly)
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

func getParam(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}
func getRawIdFromRequest(r *http.Request, name string) string {
	rawId := getRawIdFromPath(r, name)
	if rawId != "" {
		return rawId
	}
	return getRawIdFromJwt(r, name)
}
func getRawIdFromPath(r *http.Request, name string) string {
	return getParam(r, name)
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
	_, _ = w.Write([]byte("notifications is alive (go)\n"))
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
		if _, is400 := err.(notifications.ValidationError); is400 {
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

func (h *httpController) getNotifications(w http.ResponseWriter, r *http.Request) {
	n, err := h.nm.GetUserNotifications(
		r.Context(),
		getId(r, "userId"),
	)
	respond(w, r, http.StatusOK, n, err, "cannot get notifications")
}

type addNotificationRequestBody struct {
	notifications.Notification
	ForceCreate bool `json:"forceCreate"`
}

func (h *httpController) addNotification(w http.ResponseWriter, r *http.Request) {
	var requestBody addNotificationRequestBody
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.nm.AddNotification(
		r.Context(),
		getId(r, "userId"),
		requestBody.Notification,
		requestBody.ForceCreate,
	)
	respond(w, r, http.StatusOK, nil, err, "cannot add notification")
}

func (h *httpController) removeNotificationById(w http.ResponseWriter, r *http.Request) {
	err := h.nm.RemoveNotificationById(
		r.Context(),
		getId(r, "userId"),
		getId(r, "notificationId"),
	)
	respond(w, r, http.StatusOK, nil, err, "cannot remove notification by id")
}

type removeNotificationByKeyRequestBody struct {
	Key string `json:"key"`
}

func (h *httpController) removeNotificationByKey(w http.ResponseWriter, r *http.Request) {
	var requestBody removeNotificationByKeyRequestBody
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.nm.RemoveNotificationByKey(
		r.Context(),
		getId(r, "userId"),
		requestBody.Key,
	)
	respond(w, r, http.StatusOK, nil, err, "cannot remove notification by key")
}

func (h *httpController) removeNotificationByKeyOnly(w http.ResponseWriter, r *http.Request) {
	notificationKey := getParam(r, "notificationKey")
	err := h.nm.RemoveNotificationByKeyOnly(
		r.Context(),
		notificationKey,
	)
	respond(w, r, http.StatusOK, nil, err, "cannot remove notification by key only")
}
