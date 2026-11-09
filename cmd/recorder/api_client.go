package main

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

func doAPIRequestBytes(ctx context.Context, client *model.Client4, method, rawURL string, data []byte, etag string) (*http.Response, error) {
	return doAPIRequestReader(ctx, client, method, rawURL, bytes.NewReader(data), map[string]string{
		model.HeaderEtagClient: etag,
	})
}

func doAPIRequest(ctx context.Context, client *model.Client4, method, rawURL, data, etag string) (*http.Response, error) {
	return doAPIRequestBytes(ctx, client, method, rawURL, []byte(data), etag)
}

func doAPIRequestReader(ctx context.Context, client *model.Client4, method, rawURL string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if client.AuthToken != "" {
		req.Header.Set(model.HeaderAuth, client.AuthType+" "+client.AuthToken)
	}
	for k, v := range client.HTTPHeader {
		req.Header.Set(k, v)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil || resp == nil {
		return resp, err
	}
	if resp.StatusCode == http.StatusNotModified {
		return resp, nil
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return resp, model.AppErrorFromJSON(resp.Body)
	}

	return resp, nil
}
