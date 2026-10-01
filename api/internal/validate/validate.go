// Package validate checks user input that ends up in Kubernetes object
// names, GitHub repo names, Vault paths and environment variables.
package validate

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxSlugLen keeps derived names within limits: "<slug>-staging" must fit a
// 63-character namespace and "kuberlaunch-<slug>" a Vault role name, with room
// for Helm/Kubernetes suffixes on object names.
const MaxSlugLen = 30

// slugRe is RFC 1035 (Service names must start with a letter).
var slugRe = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

// reservedSlugs collide with platform objects: the ArgoCD "default" project,
// the Vault roles kuberlaunch-api / kuberlaunch-platform-secrets, and the
// platform's own components.
var reservedSlugs = map[string]bool{
	"default": true, "api": true, "web": true, "platform-secrets": true,
	"kuberlaunch": true, "argocd": true, "vault": true, "monitoring": true,
	"kube": true, "kube-system": true, "external-secrets": true, "ingress-nginx": true,
}

func Slug(slug string) error {
	switch {
	case len(slug) < 2:
		return fmt.Errorf("slug %q must be at least 2 characters", slug)
	case len(slug) > MaxSlugLen:
		return fmt.Errorf("slug %q must be at most %d characters", slug, MaxSlugLen)
	case !slugRe.MatchString(slug) || strings.Contains(slug, "--"):
		return fmt.Errorf("slug %q must start with a letter and contain only lowercase letters, digits and single hyphens", slug)
	case reservedSlugs[slug]:
		return fmt.Errorf("slug %q is reserved", slug)
	}
	return nil
}

// secretKeyRe matches names usable as environment variables (envFrom).
var secretKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const (
	MaxSecretKeyLen   = 128
	MaxSecretValueLen = 64 * 1024
)

func SecretKey(key string) error {
	if len(key) == 0 || len(key) > MaxSecretKeyLen || !secretKeyRe.MatchString(key) {
		return fmt.Errorf("secret key %q must be a valid environment variable name (letters, digits, underscore; not starting with a digit)", key)
	}
	return nil
}

func SecretValue(value string) error {
	if len(value) > MaxSecretValueLen {
		return fmt.Errorf("secret value must be at most %d bytes", MaxSecretValueLen)
	}
	return nil
}
