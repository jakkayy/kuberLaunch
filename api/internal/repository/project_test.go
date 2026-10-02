package repository

import "testing"

func TestToSlug(t *testing.T) {
	cases := map[string]string{
		"User Service":       "user-service",
		"  my__api  ":        "my-api",
		"a--b":               "a-b",
		"-Leading-":          "leading",
		"Café API!":          "caf-api",
		"Order Service v2.1": "order-service-v21",
	}
	for in, want := range cases {
		if got := ToSlug(in); got != want {
			t.Errorf("ToSlug(%q) = %q, want %q", in, got, want)
		}
	}
}
