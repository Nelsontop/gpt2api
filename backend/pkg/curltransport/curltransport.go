// Package curltransport 提供 curl-based HTTP RoundTripper。
// Cloudflare JA4 TLS 指纹检测拦截 Go/uTLS 的请求(403)，
// 只有真实浏览器 TLS（curl OpenSSL 3.5.6）能通过。
// 此 RoundTripper 用 curl subprocess 发送 HTTP 请求，
// 使 Go 的 http.Client 能通过 Cloudflare。
package curltransport

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Transport 实现 http.RoundTripper，内部用 curl 发送请求。
type Transport struct {
	ProxyURL string
	Timeout  time.Duration
}

// RoundTrip 用 curl 发送 HTTP 请求并返回响应。
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx := req.Context()
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		timeout = time.Until(deadline)
		if timeout < 5*time.Second {
			timeout = 5 * time.Second
		}
	}

	args := []string{
		"-s",
		"-w", "\n__CURL_STATUS__%{http_code}\n__CURL_SIZE__%{size_download}",
		"--max-time", strconv.Itoa(int(timeout.Seconds())),
	}

	if t.ProxyURL != "" {
		args = append(args, "--proxy", t.ProxyURL)
	}

	// Method
	args = append(args, "-X", req.Method)

	// Headers
	for k, v := range req.Header {
		for _, vv := range v {
			args = append(args, "-H", fmt.Sprintf("%s: %s", k, vv))
		}
	}

	// Body
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("curltransport read body: %w", err)
		}
		req.Body.Close()
		if len(body) > 0 {
			args = append(args, "-d", string(body))
		}
	}

	args = append(args, req.URL.String())

	cmd := exec.CommandContext(ctx, "curl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("curltransport: %w (output: %s)", err, snippet(out, 200))
	}

	raw := string(out)
	statusCode := 0
	contentLength := 0
	bodyEnd := len(raw)

	// Parse status code
	statusIdx := strings.LastIndex(raw, "__CURL_STATUS__")
	if statusIdx >= 0 {
		statusStr := raw[statusIdx+len("__CURL_STATUS__"):]
		rest := statusStr
		newlineIdx := strings.Index(rest, "\n")
		if newlineIdx >= 0 {
			statusStr = rest[:newlineIdx]
			rest = rest[newlineIdx+1:]
		}
		statusCode, _ = strconv.Atoi(strings.TrimSpace(statusStr))
		bodyEnd = statusIdx

		// Parse content size
		sizeIdx := strings.Index(rest, "__CURL_SIZE__")
		if sizeIdx >= 0 {
			sizeStr := rest[sizeIdx+len("__CURL_SIZE__"):]
			newlineIdx2 := strings.Index(sizeStr, "\n")
			if newlineIdx2 >= 0 {
				sizeStr = sizeStr[:newlineIdx2]
			}
			contentLength, _ = strconv.Atoi(strings.TrimSpace(sizeStr))
		}
	}

	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	respBody := io.NopCloser(bytes.NewReader([]byte(raw[:bodyEnd])))
	if contentLength > 0 {
		respBody = io.NopCloser(io.LimitReader(bytes.NewReader([]byte(raw[:bodyEnd])), int64(contentLength)))
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       respBody,
	}
	return resp, nil
}

func snippet(b []byte, limit int) string {
	s := string(b)
	if limit > 0 && len(s) > limit {
		return s[:limit]
	}
	return s
}

// NewClient 创建使用 curl transport 的 http.Client。
func NewClient(proxyURL string, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 6 * time.Minute
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &Transport{ProxyURL: proxyURL, Timeout: timeout},
	}
}

// Cache wraps Transport with response body caching so multiple reads work.
type Cache struct {
	mu    sync.Mutex
	cache map[string]*cachedResponse
}

type cachedResponse struct {
	statusCode int
	header     http.Header
	body       []byte
}

// CachedTransport caches responses so body can be read multiple times.
type CachedTransport struct {
	Inner *Transport
}

func (ct *CachedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := ct.Inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}