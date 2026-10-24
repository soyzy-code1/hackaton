package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/apperr"
	"invoicekeeper/internal/auth"
	"invoicekeeper/internal/config"
	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

type ctxKey string

const userKey ctxKey = "user"

func writeAppError(c *gin.Context, e *apperr.AppError) {
	c.JSON(e.Status, gin.H{"detail": e.Message, "code": e.Code})
}

func authMiddleware(cfg config.Config, st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		if cookie, err := c.Request.Cookie("access_token"); err == nil {
			// Browser sessions authenticate through the HttpOnly cookie.
			token = strings.TrimSpace(cookie.Value)
		}
		if token == "" {
			// Internal service clients use the dedicated header; this is not a browser JWT path.
			token = strings.TrimSpace(c.GetHeader("X-Internal-Token"))
		}
		// Service-to-service access (notification bot): the shared internal token
		// is accepted either in X-Internal-Token or as a Bearer token and acts on
		// behalf of the bootstrap administrator account.
		if cfg.InternalAPIToken != "" {
			internal := strings.TrimSpace(c.GetHeader("X-Internal-Token"))
			if internal == cfg.InternalAPIToken || token == cfg.InternalAPIToken {
				user, err := st.GetUserByLogin(cfg.AdminLogin)
				if err != nil || user == nil || !user.IsActive {
					writeAppError(c, apperr.New(http.StatusUnauthorized, "Service user not found or inactive", "AUTH_REQUIRED"))
					c.Abort()
					return
				}
				c.Set(string(userKey), user)
				c.Next()
				return
			}
		}
		if token == "" {
			writeAppError(c, apperr.AuthRequired)
			c.Abort()
			return
		}
		claims, err := auth.ParseToken(token, cfg.SecretKey)
		if err != nil {
			writeAppError(c, apperr.New(http.StatusUnauthorized, "Invalid or expired token", "AUTH_REQUIRED"))
			c.Abort()
			return
		}
		if claims.JTI != "" && st.IsTokenRevoked(claims.JTI) {
			writeAppError(c, apperr.New(http.StatusUnauthorized, "Token revoked", "AUTH_REQUIRED"))
			c.Abort()
			return
		}
		userID, err := strconv.Atoi(claims.Subject)
		if err != nil {
			writeAppError(c, apperr.AuthRequired)
			c.Abort()
			return
		}
		user, err := st.GetUserByID(userID)
		if err != nil || user == nil || !user.IsActive {
			writeAppError(c, apperr.New(http.StatusUnauthorized, "User not found or inactive", "AUTH_REQUIRED"))
			c.Abort()
			return
		}
		c.Set(string(userKey), user)
		c.Next()
	}
}

func requireAdmin(c *gin.Context) {
	u, ok := currentUser(c)
	if !ok {
		writeAppError(c, apperr.AuthRequired)
		c.Abort()
		return
	}
	if u.Role != "admin" {
		writeAppError(c, apperr.Forbidden)
		c.Abort()
		return
	}
	c.Next()
}

func currentUser(c *gin.Context) (*models.User, bool) {
	v, ok := c.Get(string(userKey))
	if !ok {
		return nil, false
	}
	u, ok := v.(*models.User)
	return u, ok
}

func (s *Server) authTokenFromRequest(c *gin.Context) string {
	if cookie, err := c.Request.Cookie("access_token"); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func internalTokenMiddleware(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Internal-Token") != cfg.InternalAPIToken {
			writeAppError(c, apperr.New(http.StatusUnauthorized, "Invalid internal token", "AUTH_REQUIRED"))
			c.Abort()
			return
		}
		c.Next()
	}
}
