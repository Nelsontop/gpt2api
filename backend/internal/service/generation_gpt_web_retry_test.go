package service

import (
	"errors"
	"testing"

	"github.com/kleinai/backend/internal/model"
)

func TestGPTWebChallengeIsRetryable(t *testing.T) {
	err := errors.New(`gpt image2 web bootstrap 403: <html><head><meta name="viewport" content="width=device-width, initial-scale=1" /></head></html>`)

	if !retryableProviderError(err) {
		t.Fatal("expected gpt web challenge to be retryable")
	}
}

func TestGPTWebChallengeIsTransientPathError(t *testing.T) {
	err := errors.New(`gpt image2 web bootstrap 403: <html><head><meta name="viewport" content="width=device-width, initial-scale=1" /></head></html>`)

	if !isTransientProviderPathError(model.ProviderGPT, err) {
		t.Fatal("expected gpt web challenge to be treated as transient path error")
	}
}

func TestGPTWebChallengeHasNoCooldown(t *testing.T) {
	err := errors.New(`gpt image2 web bootstrap 403: <html><head><meta name="viewport" content="width=device-width, initial-scale=1" /></head></html>`)

	if got := providerCooldown(err); got != 0 {
		t.Fatalf("expected zero cooldown for gpt web challenge, got %s", got)
	}
}

func TestUserFacingGenerationErrorForGPTWebChallenge(t *testing.T) {
	got := userFacingGenerationError(`provider call: gpt image2 web bootstrap 403: <html><head><meta name="viewport" content="width=device-width, initial-scale=1" /></head></html>`)

	want := "ChatGPT Web 触发风控挑战，请更换可用代理或账号后再试"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
