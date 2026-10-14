package jwt

import (
	"context"
	"marketplace/gateway/pkg/response"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type userIDKey struct{}

func UserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey{}).(string)
	return id
}

func writeAuthError(w http.ResponseWriter, code int, name, message string) {
	_ = response.Body(w, struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}{name, message}, code)
}

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
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "access token is required")
			return
		}

		bearer := strings.Fields(authorization)
		if len(bearer) != 2 || bearer[0] != "Bearer" {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "invalid authorization token")
			return
		}

		claims, err := m.jwtManager.Parse(bearer[1])
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "invalid access token")
			return
		}
		if claims.TokenType != "access" {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "access token is required")
			return
		}
		if _, err := uuid.Parse(claims.Subject); err != nil || len(claims.Subject) != 36 {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "invalid user ID")
			return
		}
		if claims.Role != role {
			writeAuthError(w, http.StatusForbidden, "forbidden", "role is not permitted")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, claims.Subject)))
	}
}
