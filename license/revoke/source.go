package revoke

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
)

// RevocationSource abstracts where the signed revocation list comes from.
type RevocationSource interface {
	Fetch(ctx context.Context) ([]byte, error)
}

// SourceFunc lets callers plug a closure as a source.
type SourceFunc func(ctx context.Context) ([]byte, error)

func (f SourceFunc) Fetch(ctx context.Context) ([]byte, error) { return f(ctx) }

func HTTPSource(url string, client *http.Client) RevocationSource {
	if client == nil {
		client = http.DefaultClient
	}
	return SourceFunc(func(ctx context.Context) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return nil, errors.New("revoke: HTTP " + resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	})
}

func FileSource(path string) RevocationSource {
	return SourceFunc(func(ctx context.Context) ([]byte, error) {
		return os.ReadFile(path)
	})
}

func EmbedSource(data []byte) RevocationSource {
	return SourceFunc(func(ctx context.Context) ([]byte, error) {
		return append([]byte(nil), data...), nil
	})
}

func MultiSource(sources ...RevocationSource) RevocationSource {
	return SourceFunc(func(ctx context.Context) ([]byte, error) {
		var lastErr error
		for _, s := range sources {
			b, err := s.Fetch(ctx)
			if err == nil {
				return b, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			lastErr = errors.New("revoke: no sources")
		}
		return nil, lastErr
	})
}
