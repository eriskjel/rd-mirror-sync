package rdapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ClientConfig struct {
	BaseURL        string
	HTTPTimeout    time.Duration
	MaxRetries     int
	RetryBase      time.Duration
	RetryMaxJitter time.Duration
	PageLimit      int
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	cfg        ClientConfig
	rng        *rand.Rand
	rngMu      sync.Mutex
}

func NewClient(cfg ClientConfig) *Client {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   8 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   8 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		cfg:     cfg,
		httpClient: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: tr,
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (c *Client) ListAllTorrents(ctx context.Context, token string) ([]Torrent, error) {
	var all []Torrent
	for page := 1; ; page++ {
		u, _ := url.Parse(c.baseURL + "/torrents")
		q := u.Query()
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("limit", fmt.Sprintf("%d", c.cfg.PageLimit))
		u.RawQuery = q.Encode()

		var batch []Torrent
		if err := c.getJSONWithRetry(ctx, token, u.String(), &batch); err != nil {
			return nil, fmt.Errorf("list torrents page=%d: %w", page, err)
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		if len(batch) < c.cfg.PageLimit {
			break
		}
	}
	return all, nil
}

func (c *Client) AddMagnetByHash(ctx context.Context, token, hash string) (string, error) {
	form := url.Values{}
	form.Set("magnet", "magnet:?xt=urn:btih:"+strings.TrimSpace(hash))
	var out addMagnetResponse
	if err := c.postFormJSONWithRetry(ctx, token, c.baseURL+"/torrents/addMagnet", form, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("addMagnet returned empty id")
	}
	return out.ID, nil
}

func (c *Client) SelectFilesAll(ctx context.Context, token, torrentID string) error {
	form := url.Values{}
	form.Set("files", "all")
	return c.postFormNoBodyWithRetry(ctx, token, c.baseURL+"/torrents/selectFiles/"+url.PathEscape(torrentID), form)
}

func (c *Client) DeleteTorrent(ctx context.Context, token, torrentID string) error {
	return c.deleteWithRetry(ctx, token, c.baseURL+"/torrents/delete/"+url.PathEscape(torrentID))
}

func (c *Client) getJSONWithRetry(ctx context.Context, token, endpoint string, out any) error {
	return c.withRetry(ctx, "GET "+endpoint, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return retryable(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return retryable(fmt.Errorf("status=%d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return retryable(err)
		}
		return nil
	})
}

func (c *Client) postFormJSONWithRetry(ctx context.Context, token, endpoint string, form url.Values, out any) error {
	return c.withRetry(ctx, "POST "+endpoint, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return retryable(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return retryable(fmt.Errorf("status=%d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return retryable(err)
		}
		return nil
	})
}

func (c *Client) postFormNoBodyWithRetry(ctx context.Context, token, endpoint string, form url.Values) error {
	return c.withRetry(ctx, "POST "+endpoint, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return retryable(err)
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return retryable(fmt.Errorf("status=%d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})
}

func (c *Client) deleteWithRetry(ctx context.Context, token, endpoint string) error {
	return c.withRetry(ctx, "DELETE "+endpoint, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return retryable(err)
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return retryable(fmt.Errorf("status=%d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})
}

type retryErr struct{ err error }

func (e retryErr) Error() string { return e.err.Error() }
func retryable(err error) error  { return retryErr{err: err} }

func isRetryable(err error) bool {
	var re retryErr
	return errors.As(err, &re)
}

func (c *Client) withRetry(ctx context.Context, op string, fn func() error) error {
	var last error
	for attempt := 1; attempt <= c.cfg.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		last = err
		if !isRetryable(err) || attempt == c.cfg.MaxRetries {
			break
		}

		backoff := c.cfg.RetryBase * time.Duration(1<<(attempt-1))
		wait := backoff + c.randomJitter(c.cfg.RetryMaxJitter)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return fmt.Errorf("%s failed after retries: %w", op, last)
}

func (c *Client) randomJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	c.rngMu.Lock()
	defer c.rngMu.Unlock()
	return time.Duration(c.rng.Int63n(int64(max) + 1))
}
