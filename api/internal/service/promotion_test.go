package service

import (
	"strings"
	"testing"

	"github.com/jakkayy/kuberlauncher/api/internal/model"
)

func TestImageTagParsing(t *testing.T) {
	values := "# staging\nreplicaCount: 1\n\nimage:\n  tag: \"sha-abc1234\"\n\ningress:\n  host: demo-staging.local\n"
	if got := parseImageTag(values); got != "sha-abc1234" {
		t.Fatalf("parseImageTag = %q", got)
	}
	if got := parseImageTag("image:\n  tag: latest # default\n"); got != "latest" {
		t.Fatalf("unquoted tag = %q", got)
	}
	updated, ok := setImageTag(values, "sha-def5678")
	if !ok || parseImageTag(updated) != "sha-def5678" {
		t.Fatalf("setImageTag = %q, %v", updated, ok)
	}
	if want := "image:\n  tag: \"sha-def5678\"\n"; !containsStr(updated, want) {
		t.Fatalf("indentation not preserved:\n%s", updated)
	}
	if _, ok := setImageTag("replicaCount: 1\n", "x"); ok {
		t.Fatal("setImageTag should fail without a tag line")
	}
}

func TestPromotionSource(t *testing.T) {
	cases := map[model.EnvName]model.EnvName{model.EnvStaging: model.EnvDev, model.EnvProd: model.EnvStaging}
	for to, want := range cases {
		if got, ok := model.PromotionSource(to); !ok || got != want {
			t.Errorf("PromotionSource(%s) = %s, %v", to, got, ok)
		}
	}
	if _, ok := model.PromotionSource(model.EnvDev); ok {
		t.Error("dev has no promotion source")
	}
	if !model.EnvProd.RequiresApproval() || model.EnvStaging.RequiresApproval() {
		t.Error("only prod requires approval")
	}
}

func containsStr(s, sub string) bool { return strings.Contains(s, sub) }
