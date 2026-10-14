package jwt

import (
	"net/http"
	"strings"
)

type Middleware struct {
	jwtManager Manager
}

func NewJWTMiddleware(jwtManager *Manager) *Middleware {
	return &Middleware{
		jwtManager: *jwtManager,
	}
}

func (m *Middleware) CheckUser(next http.Handler, role string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			http.Error(w, "No authorization token", http.StatusForbidden)
			return
		}

		bearer := strings.Split(authorization, " ")
		if len(bearer) != 2 || bearer[0] != "Bearer" {
			http.Error(w, "Invalid authorization token", http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtManager.Parse(bearer[1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if claims.TokenType != "access" {
			http.Error(w, "{error: Authorization provide no permission for the request}", http.StatusMethodNotAllowed)
			return
		}
		if claims.Role != role {
			http.Error(w, "{error: Authorization provide no permission for the request}", http.StatusMethodNotAllowed)
			return
		}

		next.ServeHTTP(w, r)
	}
}
