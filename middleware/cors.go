package middleware

import "github.com/AdhityaRamadhanus/fasthttpcors"

var Cors MiddleWare

func init() {
	corsHandler := fasthttpcors.NewCorsHandler(fasthttpcors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	})
	Cors = corsHandler.CorsMiddleware
}
