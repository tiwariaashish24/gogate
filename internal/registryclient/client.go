package regclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type registerRequest struct {
	Service string `json:"service"`
	Address string `json:"address"`
}

func Register(ctx context.Context, baseURL, service, address string) error {
	body, err := json.Marshal(registerRequest{Service: service, Address: address})
	if err != nil {
		return err
	}

	url := strings.TrimRight(baseURL, "/") + "/registry/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}
	return nil
}

func RegisterWithRetry(ctx context.Context, baseURL, service, address string, interval time.Duration) {
	for {
		if err := Register(ctx, baseURL, service, address); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
