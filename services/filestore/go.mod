module github.com/das7pad/overleaf-go/services/filestore

go 1.16

require (
	github.com/das7pad/overleaf-go v0.0.0
	github.com/gorilla/mux v1.8.0
	github.com/minio/minio-go/v7 v7.0.10
	go.mongodb.org/mongo-driver v1.5.1
)

replace (
	github.com/das7pad/overleaf-go v0.0.0 => ../../
)
