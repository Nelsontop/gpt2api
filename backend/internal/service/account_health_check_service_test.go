package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoRegisterRequestJSON(t *testing.T) {
	req := autoRegisterRequest{Count: 10, Workers: 1}
	body, err := json.Marshal(req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"count":10,"workers":1}`, string(body))

	var decoded autoRegisterRequest
	err = json.NewDecoder(bytes.NewReader(body)).Decode(&decoded)
	require.NoError(t, err)
	assert.Equal(t, 10, decoded.Count)
	assert.Equal(t, 1, decoded.Workers)
}

func TestAutoRegisterHTTPCallFormat(t *testing.T) {
	var (
		receivedBody autoRegisterRequest
		requestMu    sync.Mutex
		callCount    int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		defer requestMu.Unlock()
		callCount++

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/auto-register-sync", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req autoRegisterRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		receivedBody = req

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Simulate the autoRegisterIfLow HTTP call with < 10 threshold
	body, _ := json.Marshal(autoRegisterRequest{Count: 10, Workers: 1})
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/auto-register-sync", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	requestMu.Lock()
	assert.Equal(t, 1, callCount)
	assert.Equal(t, 10, receivedBody.Count)
	assert.Equal(t, 1, receivedBody.Workers)
	requestMu.Unlock()
}

func TestAutoRegisterThresholdLogic(t *testing.T) {
	// Verify threshold constant
	assert.Equal(t, 10, autoRegisterThreshold)

	// Below threshold triggers
	assert.True(t, 3 < autoRegisterThreshold)
	assert.True(t, 9 < autoRegisterThreshold)

	// At or above threshold does NOT trigger
	assert.False(t, 10 < autoRegisterThreshold)
	assert.False(t, 15 < autoRegisterThreshold)
}

func TestAutoRegisterHandlesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	body, _ := json.Marshal(autoRegisterRequest{Count: 10, Workers: 1})
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/auto-register-sync", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}