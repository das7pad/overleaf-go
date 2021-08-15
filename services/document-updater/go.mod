module github.com/das7pad/overleaf-go/services/document-updater

go 1.16

require (
	github.com/das7pad/overleaf-go v0.0.0
	github.com/go-redis/redis/v8 v8.8.0
	github.com/gorilla/mux v1.8.0
	github.com/sergi/go-diff v1.2.0
	go.mongodb.org/mongo-driver v1.5.1
)

replace (
	github.com/das7pad/overleaf-go v0.0.0 => ../../
)
