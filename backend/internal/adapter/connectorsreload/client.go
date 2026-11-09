package connectorsreload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client POSTs to syslog-ng reload agent.
type Client struct {
	URL    string
	Secret string
	Client *http.Client
}

func New(url, secret string) *Client {
	return &Client{
		URL:    strings.TrimSpace(url),
		Secret: strings.TrimSpace(secret),
		Client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Reload(ctx context.Context) error {
	if c == nil || c.URL == "" {
		return fmt.Errorf("syslog reload URL is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, nil)
	if err != nil {
		return err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
		req.Header.Set("X-GA-Reload-Token", c.Secret)
	}
	cli := c.Client
	if cli == nil {
		cli = http.DefaultClient
	}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("reload HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
