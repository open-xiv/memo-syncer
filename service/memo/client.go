package memo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo-syncer/model"
	"net/http"
	"os"

	"golang.org/x/net/http2"
)

const ApiURL = "https://api.sumemo.dev"

func CreateFight(ctx context.Context, fight *model.Fight) error {
	client := &http.Client{
		Transport: &http2.Transport{},
	}

	body, err := json.Marshal(fight)
	if err != nil {
		return fmt.Errorf("failed to marshal fight: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ApiURL+"/fight", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Key", os.Getenv("MEMO_KEY"))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
