module github.com/das7pad/overleaf-go/services/chat

go 1.14

require (
	github.com/das7pad/overleaf-go v0.0.0
	github.com/gorilla/mux v1.8.0
	go.mongodb.org/mongo-driver v1.5.1
)

replace (
	github.com/das7pad/overleaf-go v0.0.0 => ../../
)
