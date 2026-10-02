package service

import "testing"

func TestPreserveImageTag(t *testing.T) {
	current := "image:\n  repository: ghcr.io/acme/demo\n  tag: \"sha-abc1234\"\n"
	generated := "image:\n  repository: ghcr.io/acme/demo\n  pullPolicy: Always\n  tag: \"latest\"\nservice: {}\n"
	want := "image:\n  repository: ghcr.io/acme/demo\n  pullPolicy: Always\n  tag: \"sha-abc1234\"\nservice: {}\n"
	if got := preserveImageTag(current, generated); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := preserveImageTag("no tag here", generated); got != generated {
		t.Fatalf("without a current tag the generated file must be unchanged, got:\n%s", got)
	}
}
