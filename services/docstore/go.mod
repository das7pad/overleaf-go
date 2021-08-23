module github.com/das7pad/overleaf-go/services/docstore

go 1.16

require (
	github.com/das7pad/overleaf-go v0.0.0
	github.com/das7pad/overleaf-go/pkg/objectStorage v0.0.0
	github.com/gorilla/mux v1.8.0
	go.mongodb.org/mongo-driver v1.7.1
)

replace (
	github.com/das7pad/overleaf-go v0.0.0 => ../../
	github.com/das7pad/overleaf-go/pkg/objectStorage v0.0.0 => ../../pkg/objectStorage
)
