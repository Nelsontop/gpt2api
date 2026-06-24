package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kleinai/backend/internal/model"
	"github.com/kleinai/backend/pkg/crypto"
)

func TestProviderCooldownGrokForbiddenIsTransient(t *testing.T) {
	err := errors.New(`grok upload HTTP 403: <!DOCTYPE html><html><head><title>Just a moment...</title></head></html>`)
	if got := providerCooldown(err); got != 0 {
		t.Fatalf("expected transient cooldown 0, got %s", got)
	}
}

func TestProviderCooldownRetryable429StillCooldowns(t *testing.T) {
	err := errors.New(`provider call: grok video HTTP 429: {"error":{"code":8,"message":"Too many requests"}}`)
	got := providerCooldown(err)
	if got < 30*time.Minute {
		t.Fatalf("expected 429 cooldown >= 30m, got %s", got)
	}
}

func TestCacheResultAssetsReturnsErrorWhenDataURLCannotBeCached(t *testing.T) {
	t.Setenv("KLEIN_STORAGE_ROOT", "/proc/klein-ai-cache-test")
	aes, err := crypto.NewAESGCM([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("new aes: %v", err)
	}
	cred, err := aes.Encrypt([]byte("session=test"))
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	svc := &GenerationService{
		aes: aes,
		cfg: &SystemConfigService{
			cache:  map[string]string{},
			loaded: time.Now(),
			ttl:    time.Hour,
		},
	}
	results := []*model.GenerationResult{{
		TaskID: "task123",
		UserID: 1,
		Kind:   "image",
		URL:    "data:image/png;base64,aGk=",
	}}

	err = svc.cacheResultAssets(
		context.Background(),
		&model.GenerationTask{TaskID: "task123"},
		&model.Account{CredentialEnc: cred},
		results,
	)

	if err == nil {
		t.Fatal("expected cache error")
	}
	if strings.HasPrefix(results[0].URL, "data:") {
		t.Fatalf("expected data URL to be rejected before DB write, got %q", results[0].URL)
	}
}

func TestIsGPTAuth401Error(t *testing.T) {
	tests := []struct {
		errMsg string
		want   bool
	}{
		{"gpt image2 web requirements 401: unauthorized", true},
		{"gpt image2 web bootstrap 401: token expired", true},
		{"gpt image2 web prepare 401: access denied", true},
		{"gpt image2 web conversation 401: auth fail", true},
		{"gpt image2 401: unauthorized", true},
		{"gpt 401: unauthorized", true},
		{"gpt image2 web bootstrap 403: <html>challenge</html>", false},
		{"grok video HTTP 401: unauthorized", false},
		{"", false},
	}
	for _, tt := range tests {
		err := errors.New(tt.errMsg)
		if got := isGPTAuth401Error(err); got != tt.want {
			t.Errorf("isGPTAuth401Error(%q) = %v, want %v", tt.errMsg, got, tt.want)
		}
	}
	if isGPTAuth401Error(nil) {
		t.Error("isGPTAuth401Error(nil) should be false")
	}
}

func TestGPTAuth401IsRetryable(t *testing.T) {
	err := errors.New("gpt image2 web requirements 401: unauthorized")
	if !retryableProviderError(err) {
		t.Fatal("expected gpt web 401 to be retryable (try other accounts)")
	}
	codexErr := errors.New("gpt image2 401: unauthorized")
	if !retryableProviderError(codexErr) {
		t.Fatal("expected gpt codex 401 to be retryable (try other accounts)")
	}
	legacyErr := errors.New("gpt 401: unauthorized")
	if !retryableProviderError(legacyErr) {
		t.Fatal("expected gpt legacy 401 to be retryable (try other accounts)")
	}
}

func TestIsTokenRevokedError(t *testing.T) {
	tests := []struct {
		errMsg string
		want   bool
	}{
		{"conversation/init HTTP 401: {\"error\":{\"message\":\"Encountered invalidated oauth token for user, failing request\",\"code\":\"token_revoked\"}}", true},
		{"conversation/init HTTP 401: token_revoked", true},
		{"Encountered invalidated oauth token for user", true},
		{"conversation/init HTTP 403: Cloudflare challenge", false},
		{"gpt image2 web 429: too many requests", false},
		{"", false},
	}
	for _, tt := range tests {
		err := errors.New(tt.errMsg)
		if got := isTokenRevokedError(err); got != tt.want {
			t.Errorf("isTokenRevokedError(%q) = %v, want %v", tt.errMsg, got, tt.want)
		}
	}
	if isTokenRevokedError(nil) {
		t.Error("isTokenRevokedError(nil) should be false")
	}
}
