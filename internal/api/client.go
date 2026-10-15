// Package api is the HTTP client for the aion-theory platform's sync
// surface (device flow + combat log ingestion, US-COMBAT-001).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Errors the caller branches on. Anything else is unexpected and surfaced as-is.
var (
	ErrUnauthorized = errors.New("the stored credential was rejected")
	ErrExpired      = errors.New("the device grant expired before it was approved")
	ErrNotFound     = errors.New("the device grant is not known to the server")
	ErrAlreadyUsed  = errors.New("the device code was already used")
	ErrTooLarge     = errors.New("the combat log is larger than the server accepts")

	// ErrRateLimited is what errors.Is sees under a RateLimitedError — a
	// bulk first sync can be hundreds of uploads back to back, and the
	// server is within its rights to 429 that; the caller branches on
	// this the same way it does on ErrUnauthorized, then backs off using
	// the RetryAfter the error carries.
	ErrRateLimited = errors.New("rate limited by the server")
)

// RateLimitedError is what UploadCombatLog returns for a 429. RetryAfter is
// the server's own Retry-After header, parsed to a time.Duration; it is 0
// when the server sent none or sent something unparseable, in which case
// the caller is expected to fall back to its own default backoff.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited by the server, retry after %v", e.RetryAfter)
	}
	return "rate limited by the server"
}

func (e *RateLimitedError) Unwrap() error { return ErrRateLimited }

// parseRetryAfter reads a Retry-After header value in the delay-seconds
// form (the only form this platform is known to send); an HTTP-date value
// or an empty/malformed header both yield 0, matched by RateLimitedError's
// doc comment above.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	seconds, err := strconv.Atoi(header)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type DeviceGrant struct {
	GrantID         string `json:"grantId"`
	UserCode        string `json:"userCode"`
	VerificationURL string `json:"verificationUrl"`
	ExpiresAt       string `json:"expiresAt"`
}

// OpenDevice starts a device authorization grant: POST /api/sync/device.
func (c *Client) OpenDevice(ctx context.Context) (DeviceGrant, error) {
	var grant DeviceGrant
	_, err := c.do(ctx, http.MethodPost, "/api/sync/device", "", nil, &grant)
	return grant, err
}

type TokenPoll struct {
	Status     string `json:"status"`
	Credential string `json:"credential,omitempty"`
}

// PollToken asks whether a device grant has been approved yet: POST /api/sync/token.
// ErrExpired, ErrNotFound and ErrAlreadyUsed name the terminal outcomes that
// stop polling; any other error is worth retrying.
func (c *Client) PollToken(ctx context.Context, grantID string) (TokenPoll, error) {
	body, err := json.Marshal(map[string]string{"grantId": grantID})
	if err != nil {
		return TokenPoll{}, err
	}
	var poll TokenPoll
	status, err := c.do(ctx, http.MethodPost, "/api/sync/token", "", bytes.NewReader(body), &poll)
	if err != nil {
		switch status {
		case http.StatusGone:
			return TokenPoll{}, ErrExpired
		case http.StatusNotFound:
			return TokenPoll{}, ErrNotFound
		case http.StatusConflict:
			return TokenPoll{}, ErrAlreadyUsed
		}
		return TokenPoll{}, err
	}
	return poll, nil
}

// RecordedMeterLogIDs lists the member's already-synced meter log ids, so
// the caller skips what the server already has (F3, D9): GET /api/sync/combat-logs/ids.
func (c *Client) RecordedMeterLogIDs(ctx context.Context, credential string) (map[string]bool, error) {
	var payload struct {
		MeterLogIDs []string `json:"meterLogIds"`
	}
	_, err := c.do(ctx, http.MethodGet, "/api/sync/combat-logs/ids", credential, nil, &payload)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(payload.MeterLogIDs))
	for _, id := range payload.MeterLogIDs {
		ids[id] = true
	}
	return ids, nil
}

// Character is one of the member's declared, ACTIVE characters, as listed
// by GET /api/sync/characters. Server is empty when the server's own
// resolution of it failed — the endpoint sends serverName: null for that
// case, which json.Decode leaves as the zero value here.
type Character struct {
	CharacterID   string `json:"characterId"`
	CharacterName string `json:"characterName"`
	ServerName    string `json:"serverName"`
}

// Characters lists the member's declared, active characters:
// GET /api/sync/characters. ErrNotFound surfaces when the server's combat
// log sync feature is disabled, same as the other sync routes.
func (c *Client) Characters(ctx context.Context, credential string) ([]Character, error) {
	var payload struct {
		Characters []Character `json:"characters"`
	}
	status, err := c.do(ctx, http.MethodGet, "/api/sync/characters", credential, nil, &payload)
	if err != nil {
		if status == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return payload.Characters, nil
}

type SyncOutcome struct {
	Kind        string `json:"kind"`
	CombatLogID string `json:"combatLogId,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// UploadCombatLog posts one gzipped combat log: POST /api/sync/combat-logs.
// The server is idempotent on (member, meter log id): a retry after a
// network failure never duplicates anything (T2).
func (c *Client) UploadCombatLog(ctx context.Context, credential string, gzipBody []byte) (SyncOutcome, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/sync/combat-logs", bytes.NewReader(gzipBody))
	if err != nil {
		return SyncOutcome{}, err
	}
	req.Header.Set("content-type", "application/gzip")
	req.Header.Set("authorization", "Bearer "+credential)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return SyncOutcome{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return SyncOutcome{}, ErrUnauthorized
	case http.StatusRequestEntityTooLarge:
		return SyncOutcome{}, ErrTooLarge
	case http.StatusTooManyRequests:
		return SyncOutcome{}, &RateLimitedError{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode != http.StatusOK {
		return SyncOutcome{}, fmt.Errorf("unexpected status %d uploading combat log", resp.StatusCode)
	}

	var outcome SyncOutcome
	if err := json.NewDecoder(resp.Body).Decode(&outcome); err != nil {
		return SyncOutcome{}, err
	}
	return outcome, nil
}

// do performs one JSON request and decodes a JSON response. The credential,
// when non-empty, is sent as a bearer token; the device-flow endpoints take
// none. It returns the HTTP status alongside any error so callers can branch
// on it without re-parsing.
func (c *Client) do(ctx context.Context, method, path, credential string, body io.Reader, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if credential != "" {
		req.Header.Set("authorization", "Bearer "+credential)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return resp.StatusCode, ErrUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("unexpected status %d for %s %s", resp.StatusCode, method, path)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}
