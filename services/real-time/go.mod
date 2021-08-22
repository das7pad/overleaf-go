module github.com/das7pad/overleaf-go/services/real-time

go 1.16

require (
	github.com/auth0/go-jwt-middleware v1.0.0
	github.com/das7pad/overleaf-go v0.0.0
	github.com/form3tech-oss/jwt-go v3.2.3+incompatible
	github.com/go-redis/redis/v8 v8.11.3
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	go.mongodb.org/mongo-driver v1.7.1
)

replace github.com/das7pad/overleaf-go v0.0.0 => ../../
