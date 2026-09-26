module github.com/chonlatee11/boat-booking/services/gateway

go 1.25.7

require (
	connectrpc.com/connect v1.20.0
	github.com/chonlatee11/boat-booking/pkg v0.0.0
)

require (
	github.com/go-chi/chi/v5 v5.3.2
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/chonlatee11/boat-booking/pkg => ../../pkg

replace github.com/chonlatee11/boat-booking/gen/go => ../../gen/go
