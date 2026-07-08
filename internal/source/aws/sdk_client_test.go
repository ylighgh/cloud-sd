package aws

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewSDKClientAppliesRequestTimeout(t *testing.T) {
	client, err := newSDKClient(AccountConfig{
		Name:            "prod",
		Regions:         []string{"ap-southeast-1"},
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
	}, "ap-southeast-1", ClientOptions{RequestTimeout: 7 * time.Second})
	if err != nil {
		t.Fatalf("newSDKClient() error = %v", err)
	}

	if client.httpClient == nil {
		t.Fatal("HTTP client is nil")
	}
	if client.httpClient.Timeout != 7*time.Second {
		t.Fatalf("HTTP client timeout = %s, want 7s", client.httpClient.Timeout)
	}
}

func TestTagsOrEmptySuppressesTagReadErrors(t *testing.T) {
	tags, err := tagsOrEmptyUnlessContextDone(context.Background(), nil, errors.New("access denied"))
	if err != nil {
		t.Fatalf("tagsOrEmptyUnlessContextDone() error = %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("tags = %#v, want empty map", tags)
	}
}

func TestTagsOrEmptyReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tagsOrEmptyUnlessContextDone(ctx, nil, errors.New("access denied"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
