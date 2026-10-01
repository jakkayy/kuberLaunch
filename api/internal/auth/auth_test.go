package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

const testSecret = "0123456789abcdef0123456789abcdef"

type memUsers struct{ upserts int }

func (m *memUsers) Upsert(_ context.Context, u *model.User) error {
	m.upserts++
	u.ID = "u1"
	return nil
}

func sign(t *testing.T, secret string, mutate func(*Claims)) string {
	t.Helper()
	c := &Claims{
		Login: "octocat",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "583231",
			Issuer:    Issuer,
			Audience:  jwt.ClaimStrings{Audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	if mutate != nil {
		mutate(c)
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func serve(a *Authenticator, header string) (*httptest.ResponseRecorder, *model.User) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var got *model.User
	r.GET("/x", a.Middleware(), func(c *gin.Context) {
		got = UserFrom(c)
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	r.ServeHTTP(w, req)
	return w, got
}

func TestNewRejectsShortSecret(t *testing.T) {
	if _, err := New("short", &memUsers{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestMiddleware(t *testing.T) {
	users := &memUsers{}
	a, _ := New(testSecret, users)

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"not bearer", "Basic abc", http.StatusUnauthorized},
		{"garbage", "Bearer nope", http.StatusUnauthorized},
		{"wrong secret", "Bearer " + sign(t, "ffffffffffffffffffffffffffffffff", nil), http.StatusUnauthorized},
		{"expired", "Bearer " + sign(t, testSecret, func(c *Claims) {
			c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
		}), http.StatusUnauthorized},
		{"wrong audience", "Bearer " + sign(t, testSecret, func(c *Claims) { c.Audience = jwt.ClaimStrings{"other"} }), http.StatusUnauthorized},
		{"no expiry", "Bearer " + sign(t, testSecret, func(c *Claims) { c.ExpiresAt = nil }), http.StatusUnauthorized},
		{"valid", "Bearer " + sign(t, testSecret, nil), http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, u := serve(a, tc.header)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if tc.want == http.StatusOK && (u == nil || u.Login != "octocat" || u.GitHubID != 583231) {
				t.Fatalf("user = %+v", u)
			}
		})
	}
}

func TestMiddlewareCachesUserUpserts(t *testing.T) {
	users := &memUsers{}
	a, _ := New(testSecret, users)
	tok := "Bearer " + sign(t, testSecret, nil)
	serve(a, tok)
	serve(a, tok)
	if users.upserts != 1 {
		t.Fatalf("upserts = %d, want 1", users.upserts)
	}
}
