package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const apiBase = "https://slides.googleapis.com/v1"

// RefreshFunc is called when the access token is expired.
// It returns the new access token and its Unix expiry timestamp.
type RefreshFunc func() (newToken string, expiresAt int64, err error)

// Client is an authenticated Google Slides API v1 client.
type Client struct {
	token       string
	tokenExpiry int64
	refreshFn   RefreshFunc
	httpClient  *http.Client
}

// NewClient creates an authenticated Client.
// refreshFn may be nil if no token refresh is needed.
func NewClient(token string, tokenExpiry int64, refreshFn RefreshFunc) *Client {
	return &Client{
		token:       token,
		tokenExpiry: tokenExpiry,
		refreshFn:   refreshFn,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ensureToken refreshes the access token if it expires within 60 seconds.
func (c *Client) ensureToken() error {
	if c.refreshFn == nil {
		return nil
	}
	if c.tokenExpiry > 0 && time.Now().Unix() < c.tokenExpiry-60 {
		return nil
	}
	newToken, expiresAt, err := c.refreshFn()
	if err != nil {
		return fmt.Errorf("refreshing token: %w", err)
	}
	c.token = newToken
	c.tokenExpiry = expiresAt
	return nil
}

func (c *Client) do(method, rawURL string, params url.Values, body any) ([]byte, error) {
	if err := c.ensureToken(); err != nil {
		return nil, err
	}
	if params != nil {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		u.RawQuery = params.Encode()
		rawURL = u.String()
	}
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if jerr := json.Unmarshal(respBody, &errResp); jerr == nil && errResp.Error.Message != "" {
			return nil, &SlidesError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("API error %d: %s", errResp.Error.Code, errResp.Error.Message),
			}
		}
		return nil, &SlidesError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		}
	}
	return respBody, nil
}

// CreatePresentation creates a new blank presentation with the given title.
func (c *Client) CreatePresentation(title string) (*Presentation, error) {
	payload := map[string]string{"title": title}
	body, err := c.do("POST", apiBase+"/presentations", nil, payload)
	if err != nil {
		return nil, err
	}
	var p Presentation
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &p, nil
}

// GetPresentation retrieves a presentation by its ID.
func (c *Client) GetPresentation(id string) (*Presentation, error) {
	u := apiBase + "/presentations/" + url.PathEscape(id)
	body, err := c.do("GET", u, nil, nil)
	if err != nil {
		return nil, err
	}
	var p Presentation
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &p, nil
}

// GetPage retrieves a specific page (slide) from a presentation.
func (c *Client) GetPage(presID, pageID string) (*Page, error) {
	u := apiBase + "/presentations/" + url.PathEscape(presID) + "/pages/" + url.PathEscape(pageID)
	body, err := c.do("GET", u, nil, nil)
	if err != nil {
		return nil, err
	}
	var p Page
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &p, nil
}

// GetThumbnail retrieves a thumbnail for a specific page.
// width and mime are optional (pass 0 and "" to use defaults).
func (c *Client) GetThumbnail(presID, pageID string, width int64, mime string) (*Thumbnail, error) {
	u := apiBase + "/presentations/" + url.PathEscape(presID) + "/pages/" + url.PathEscape(pageID) + "/thumbnail"
	params := url.Values{}
	if width > 0 {
		params.Set("thumbnailProperties.thumbnailSize", "CUSTOM")
	}
	if mime != "" {
		params.Set("thumbnailProperties.mimeType", mime)
	}
	var p url.Values
	if len(params) > 0 {
		p = params
	}
	body, err := c.do("GET", u, p, nil)
	if err != nil {
		return nil, err
	}
	var t Thumbnail
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &t, nil
}

// BatchUpdate applies a batch of updates to a presentation.
// requests must be a JSON array of Request objects.
func (c *Client) BatchUpdate(presID string, requests json.RawMessage) (*BatchUpdateResponse, error) {
	u := apiBase + "/presentations/" + url.PathEscape(presID) + ":batchUpdate"
	payload := map[string]json.RawMessage{"requests": requests}
	body, err := c.do("POST", u, nil, payload)
	if err != nil {
		return nil, err
	}
	var resp BatchUpdateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}
