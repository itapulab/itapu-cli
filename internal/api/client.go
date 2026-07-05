// Package api implements the Itapu api/v1 CLI contract described in
// docs/cli-integration.md.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the Itapu web app origin (e.g. https://app.itapu.dev).
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Error is the uniform error shape: { "error": { code, message, details } }.
type Error struct {
	Status  int             `json:"-"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Code, e.Status)
}

// ProjectDetails is the `details` payload of environment_not_found and
// environment_access_denied errors.
type ProjectDetails struct {
	EnvironmentSlug string `json:"environmentSlug"`
	Projects        []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"projects"`
}

func (c *Client) do(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var wrapper struct {
			Error *Error `json:"error"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && wrapper.Error != nil {
			wrapper.Error.Status = resp.StatusCode
			return wrapper.Error
		}
		return &Error{Status: resp.StatusCode, Code: "unknown_error",
			Message: fmt.Sprintf("unexpected response (HTTP %d)", resp.StatusCode)}
	}

	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("invalid response from server: %w", err)
		}
	}
	return nil
}

// ---- itapu login ----

type LoginStartResponse struct {
	DeviceCode          string    `json:"deviceCode"`
	UserCode            string    `json:"userCode"`
	VerificationUrl     string    `json:"verificationUrl"`
	ExpiresAt           time.Time `json:"expiresAt"`
	PollIntervalSeconds int       `json:"pollIntervalSeconds"`
}

func (c *Client) LoginStart() (*LoginStartResponse, error) {
	var out LoginStartResponse
	if err := c.do("POST", "/api/v1/cli/login", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type LoginPollResponse struct {
	Status       string    `json:"status"` // pending | denied | expired | approved
	AccountToken string    `json:"accountToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}

func (c *Client) LoginPoll(deviceCode string) (*LoginPollResponse, error) {
	var out LoginPollResponse
	body := map[string]string{"deviceCode": deviceCode}
	if err := c.do("POST", "/api/v1/cli/login/poll", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- itapu init ----

type Environment struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type Project struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Environments []Environment `json:"environments"`
}

type Organization struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Role     string    `json:"role"` // owner | admin | member
	Projects []Project `json:"projects"`
}

type Workspace struct {
	Organizations []Organization `json:"organizations"`
}

func (c *Client) Workspace() (*Workspace, error) {
	var out Workspace
	if err := c.do("GET", "/api/v1/cli/workspace", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type AuthRequestResponse struct {
	RequestCode         string    `json:"requestCode"`
	ApprovalUrl         string    `json:"approvalUrl"`
	ExpiresAt           time.Time `json:"expiresAt"`
	PollIntervalSeconds int       `json:"pollIntervalSeconds"`
}

func (c *Client) CreateAuthRequest(orgID string, projectIDs []string, environmentSlug string) (*AuthRequestResponse, error) {
	var out AuthRequestResponse
	body := map[string]any{
		"orgId":           orgID,
		"projectIds":      projectIDs,
		"environmentSlug": environmentSlug,
	}
	if err := c.do("POST", "/api/v1/cli/auth-requests", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type Grant struct {
	ProjectID       string `json:"projectId"`
	ProjectName     string `json:"projectName"`
	EnvironmentID   string `json:"environmentId"`
	EnvironmentSlug string `json:"environmentSlug"`
	EnvironmentName string `json:"environmentName"`
}

type AuthRequestPollResponse struct {
	Status       string    `json:"status"`
	SecretsToken string    `json:"secretsToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	Grants       []Grant   `json:"grants,omitempty"`
}

func (c *Client) AuthRequestPoll(requestCode string) (*AuthRequestPollResponse, error) {
	var out AuthRequestPollResponse
	body := map[string]string{"requestCode": requestCode}
	if err := c.do("POST", "/api/v1/cli/auth-requests/poll", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- itapu run ----

type Secret struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SecretsResponse struct {
	Environment struct {
		ID          string `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		ProjectID   string `json:"projectId"`
		ProjectName string `json:"projectName"`
	} `json:"environment"`
	Secrets []Secret `json:"secrets"`
}

func (c *Client) Secrets(environmentID string) (*SecretsResponse, error) {
	var out SecretsResponse
	path := "/api/v1/cli/secrets?environmentId=" + url.QueryEscape(environmentID)
	if err := c.do("GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
