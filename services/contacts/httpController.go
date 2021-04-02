// Golang port of the Overleaf contacts service
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
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HttpController interface {
	GetRouter() http.Handler
}

func NewHttpController(cm ContactManager) HttpController {
	return &httpController{cm: cm}
}

type httpController struct {
	cm ContactManager
}

func (h *httpController) GetRouter() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/status", h.status)
	router.
		NewRoute().
		Methods("GET").
		Path("/user/{userId}/contacts").
		HandlerFunc(h.getContacts)
	router.
		NewRoute().
		Methods("POST").
		Path("/user/{userId}/contacts").
		HandlerFunc(h.addContacts)

	return router
}

func errorResponse(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	_, _ = w.Write([]byte(message))
}

func (h *httpController) status(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
	_, _ = w.Write([]byte("contacts is alive (go)\n"))
}

type addContactRequestBody struct {
	ContactId string `json:"contact_id"`
}

func (h *httpController) addContacts(w http.ResponseWriter, r *http.Request) {
	userId, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		errorResponse(w, 400, "invalid userId")
		return
	}
	var requestBody addContactRequestBody
	err = json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		errorResponse(w, 400, "invalid request body")
		return
	}
	contactId, err := primitive.ObjectIDFromHex(requestBody.ContactId)
	if err != nil {
		errorResponse(w, 400, "invalid userId")
		return
	}

	err = h.cm.TouchContact(r.Context(), userId, contactId)
	if err != nil {
		log.Println(err)
		errorResponse(w, 500, "cannot touch contacts of user")
		return
	}

	err = h.cm.TouchContact(r.Context(), contactId, userId)
	if err != nil {
		log.Println(err)
		errorResponse(w, 500, "cannot touch contacts of contact")
		return
	}

	w.WriteHeader(204)
}

const ContactLimit = 50

type contact struct {
	UserId  string
	Details ContactDetails
}

type getContactsResponseBody struct {
	ContactIds []string `json:"contact_ids"`
}

func (h *httpController) getContacts(w http.ResponseWriter, r *http.Request) {
	userId, err := primitive.ObjectIDFromHex(mux.Vars(r)["userId"])
	if err != nil {
		errorResponse(w, 400, "invalid userId")
		return
	}
	limit := ContactLimit
	limitQueryParam := r.URL.Query().Get("limit")
	if limitQueryParam != "" {
		limit64, err := strconv.ParseInt(limitQueryParam, 10, 64)
		if err != nil {
			errorResponse(w, 400, "invalid limit")
			return
		}
		limit = int(limit64)
		if limit > ContactLimit {
			// silently limit response size
			limit = ContactLimit
		}
	}

	contactsMap, err := h.cm.GetContacts(r.Context(), userId)
	if err != nil {
		errorResponse(w, 500, "cannot read contacts")
		return
	}

	contacts := make([]contact, 0)
	for contactId, details := range contactsMap {
		contacts = append(contacts, contact{
			UserId:  contactId,
			Details: details,
		})
	}
	sort.Slice(contacts, func(i, j int) bool {
		a := contacts[i].Details
		b := contacts[j].Details
		if a.Connections > b.Connections {
			return true
		} else if a.Connections < b.Connections {
			return false
		} else if a.LastTouched > b.LastTouched {
			return true
		} else if a.LastTouched < b.LastTouched {
			return false
		} else {
			return false
		}
	})

	responseSize := len(contacts)
	if responseSize > limit {
		responseSize = limit
	}
	contactIds := make([]string, responseSize)
	for i := 0; i < responseSize; i++ {
		contactIds[i] = contacts[i].UserId
	}
	err = json.NewEncoder(w).Encode(
		&getContactsResponseBody{ContactIds: contactIds},
	)
}
