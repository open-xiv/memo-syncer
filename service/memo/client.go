package memo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo-syncer/model"
	"memo-syncer/version"
	"net/http"
	"os"
	"sync"

	"golang.org/x/net/http2"
)

const ApiURL = "https://api.sumemo.dev"

var (
	httpClientOnce sync.Once
	httpClient     *http.Client
)

func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = &http.Client{
			Transport: &http2.Transport{},
		}
	})
	return httpClient
}

func CreateFight(ctx context.Context, fight *model.Fight) error {
	body, err := json.Marshal(fight)
	if err != nil {
		return fmt.Errorf("failed to marshal fight: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ApiURL+"/fight/", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Key", os.Getenv("MEMO_KEY"))
	req.Header.Set("X-Client-Name", version.ClientName)
	req.Header.Set("X-Client-Version", version.Current)

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, bytes.TrimSpace(respBody))
	}

	return nil
}
