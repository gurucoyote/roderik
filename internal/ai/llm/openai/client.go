package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewClient(apiKey string, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (c *Client) CreateChatCompletion(ctx context.Context, req CreateRequest) (*APIResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/chat/completions", c.baseURL),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// capture full response body for debugging
		data, _ := io.ReadAll(resp.Body)

		// try structured decode
		var errResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		if derr := json.Unmarshal(data, &errResp); derr == nil {
			return nil, fmt.Errorf(
				"OpenAI API error: %s: %s (status=%d)\nRequest URL: %s/chat/completions\nResponse body: %s",
				errResp.Error.Type,
				errResp.Error.Message,
				resp.StatusCode,
				c.baseURL,
				data,
			)
		}

		// fallback to raw
		return nil, fmt.Errorf(
			"OpenAI API error: status=%d\nRequest URL: %s/chat/completions\nResponse body: %s",
			resp.StatusCode,
			c.baseURL,
			data,
		)
	}

	var response APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &response, nil
}
