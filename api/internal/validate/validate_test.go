package validate

import (
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	valid := []string{"ab", "user-service", "a1", "svc-2", strings.Repeat("a", MaxSlugLen)}
	invalid := []string{"", "a", "1abc", "-abc", "abc-", "ab--c", "Abc", "a_b", "a.b", "default", "api",
		strings.Repeat("a", MaxSlugLen+1)}
	for _, s := range valid {
		if err := Slug(s); err != nil {
			t.Errorf("Slug(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range invalid {
		if err := Slug(s); err == nil {
			t.Errorf("Slug(%q) = nil, want error", s)
		}
	}
}

func TestSecretKey(t *testing.T) {
	for _, k := range []string{"DATABASE_URL", "_X", "a1"} {
		if err := SecretKey(k); err != nil {
			t.Errorf("SecretKey(%q) = %v", k, err)
		}
	}
	for _, k := range []string{"", "1A", "A-B", "A B", "A=B", strings.Repeat("A", MaxSecretKeyLen+1)} {
		if err := SecretKey(k); err == nil {
			t.Errorf("SecretKey(%q) = nil, want error", k)
		}
	}
}

func TestSecretValue(t *testing.T) {
	if err := SecretValue(strings.Repeat("x", MaxSecretValueLen)); err != nil {
		t.Error(err)
	}
	if err := SecretValue(strings.Repeat("x", MaxSecretValueLen+1)); err == nil {
		t.Error("expected error for oversized value")
	}
}
