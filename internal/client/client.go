package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode body: %w", err)
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, buf)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTP.Do(req)
}

func doJSON[T any](ctx context.Context, c *Client, method, path string, body any) (T, error) {
	var zero T
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return zero, &APIError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: string(b)}
	}
	if resp.ContentLength == 0 {
		return zero, nil
	}
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		if errors.Is(err, io.EOF) {
			return zero, nil
		}
		return zero, fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return out, nil
}

func Get[T any](ctx context.Context, c *Client, path string) (T, error) {
	return doJSON[T](ctx, c, http.MethodGet, path, nil)
}

func Post[T any](ctx context.Context, c *Client, path string, body any) (T, error) {
	return doJSON[T](ctx, c, http.MethodPost, path, body)
}

func Put[T any](ctx context.Context, c *Client, path string, body any) (T, error) {
	return doJSON[T](ctx, c, http.MethodPut, path, body)
}

func Delete[T any](ctx context.Context, c *Client, path string) (T, error) {
	return doJSON[T](ctx, c, http.MethodDelete, path, nil)
}

func PostBinary[T any](ctx context.Context, c *Client, path string, body io.Reader) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, body)
	if err != nil {
		return zero, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return zero, &APIError{Method: http.MethodPost, Path: path, StatusCode: resp.StatusCode, Body: string(b)}
	}
	if resp.ContentLength == 0 {
		return zero, nil
	}
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		if errors.Is(err, io.EOF) {
			return zero, nil
		}
		return zero, fmt.Errorf("decode %s %s: %w", http.MethodPost, path, err)
	}
	return out, nil
}
