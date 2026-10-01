// Package auth authenticates API requests using the session JWT issued by
// the web frontend after GitHub OAuth login.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

const (
	// Issuer and Audience must match the values the web app signs with.
	Issuer   = "kuberlaunch-web"
	Audience = "kuberlaunch-api"

	userKey = "auth.user"
)

// Claims is the session payload signed by the web app.
type Claims struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	jwt.RegisteredClaims
}

type userStore interface {
	Upsert(ctx context.Context, u *model.User) error
}

type Authenticator struct {
	secret []byte
	users  userStore

	mu   sync.Mutex
	seen map[int64]cachedUser // avoids a DB write on every request
}

type cachedUser struct {
	user    model.User
	expires time.Time
}

const userCacheTTL = 5 * time.Minute

func New(secret string, users userStore) (*Authenticator, error) {
	if len(secret) < 32 {
		return nil, errors.New("AUTH_JWT_SECRET must be at least 32 characters")
	}
	return &Authenticator{secret: []byte(secret), users: users, seen: map[int64]cachedUser{}}, nil
}

// Verify parses and validates a bearer token.
func (a *Authenticator) Verify(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return a.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(Audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if claims.Subject == "" || claims.Login == "" {
		return nil, errors.New("token missing subject or login")
	}
	return claims, nil
}

// Middleware rejects requests without a valid session token and stores the
// authenticated user in the gin context.
func (a *Authenticator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		claims, err := a.Verify(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}
		user, err := a.resolveUser(c.Request.Context(), claims)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "load user: " + err.Error()})
			return
		}
		c.Set(userKey, user)
		c.Next()
	}
}

func (a *Authenticator) resolveUser(ctx context.Context, claims *Claims) (*model.User, error) {
	githubID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid subject %q", claims.Subject)
	}

	a.mu.Lock()
	cached, ok := a.seen[githubID]
	a.mu.Unlock()
	if ok && time.Now().Before(cached.expires) && cached.user.Login == claims.Login {
		u := cached.user
		return &u, nil
	}

	u := &model.User{GitHubID: githubID, Login: claims.Login, Name: claims.Name, AvatarURL: claims.AvatarURL}
	if err := a.users.Upsert(ctx, u); err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.seen[githubID] = cachedUser{user: *u, expires: time.Now().Add(userCacheTTL)}
	a.mu.Unlock()
	return u, nil
}

// UserFrom returns the authenticated user, or nil outside the middleware.
func UserFrom(c *gin.Context) *model.User {
	if v, ok := c.Get(userKey); ok {
		return v.(*model.User)
	}
	return nil
}
