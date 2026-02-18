// Package middleware provides HTTP middleware for the SHSH API.
package middleware

import "net/http"

// CORS returns middleware that handles CORS headers.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				// Only allow credentials for explicit origins, not wildcard matches.
				// Setting Allow-Credentials with a wildcard-echoed origin enables CSRF.
				for _, o := range allowedOrigins {
					if o != "*" && o == origin {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
						break
					}
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
