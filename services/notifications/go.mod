module github.com/das7pad/overleaf-go/services/notifications

go 1.16

require (
	github.com/das7pad/overleaf-go v0.0.0
	github.com/auth0/go-jwt-middleware v1.0.0
	github.com/form3tech-oss/jwt-go v3.2.2+incompatible
	github.com/gorilla/mux v1.8.0
	go.mongodb.org/mongo-driver v1.5.1
)

replace (
	github.com/das7pad/overleaf-go v0.0.0 => ../../
)
