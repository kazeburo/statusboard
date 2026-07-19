package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestScenario_ConcurrentWorkerAndHTTPHandlers(t *testing.T) {
	opt := newTestOpt(t)
	testCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	now := time.Now()
	writeServiceLog(t, opt.Data, []*ServiceLog{
		{
			Time:         now.Add(-30 * time.Minute),
			Name:         "Google",
			CategoryName: "Web",
			Command:      []string{"ping", "google.com"},
			Status:       0,
		},
	}, now.Format("20060102"))

	if err := opt.renderStatusPage(testCtx); err != nil {
		t.Fatalf("initial renderStatusPage failed: %v", err)
	}

	e := opt.buildHandler(testCtx)
	ts := httptest.NewServer(e)
	defer ts.Close()

	const (
		workerIterations  = 24
		clientGoroutines  = 4
		requestsPerClient = 24
	)

	errCh := make(chan error, 1)
	reportErr := func(err error) {
		if err == nil {
			return
		}
		select {
		case errCh <- err:
			cancel()
		default:
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < workerIterations; i++ {
			select {
			case <-testCtx.Done():
				return
			default:
			}
			status := 0
			if i%2 == 1 {
				status = 1
			}
			err := opt.appendServiceLog(&ServiceLog{
				Time:         time.Now(),
				Name:         "Google",
				CategoryName: "Web",
				Command:      []string{"ping", "google.com"},
				Status:       status,
			})
			if err != nil {
				reportErr(fmt.Errorf("appendServiceLog failed: %w", err))
				return
			}
			if err := opt.renderStatusPage(testCtx); err != nil {
				reportErr(fmt.Errorf("renderStatusPage failed: %w", err))
				return
			}
		}
	}()

	for g := 0; g < clientGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := ts.Client()
			for i := 0; i < requestsPerClient; i++ {
				select {
				case <-testCtx.Done():
					return
				default:
				}
				path := "/_json"
				if (id+i)%2 == 0 {
					path = "/"
				}

				req, err := http.NewRequestWithContext(testCtx, http.MethodGet, ts.URL+path, nil)
				if err != nil {
					reportErr(fmt.Errorf("new request failed: %w", err))
					return
				}
				if i%3 == 0 {
					req.Header.Set("If-Modified-Since", time.Now().UTC().Format(http.TimeFormat))
				}

				resp, err := client.Do(req)
				if err != nil {
					reportErr(fmt.Errorf("http do failed: %w", err))
					return
				}

				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotModified {
					resp.Body.Close()
					reportErr(fmt.Errorf("unexpected status %d for %s", resp.StatusCode, path))
					return
				}

				if resp.StatusCode == http.StatusOK && resp.Header.Get("Last-Modified") == "" {
					resp.Body.Close()
					reportErr(fmt.Errorf("missing Last-Modified header for %s", path))
					return
				}

				if path == "/_json" && resp.StatusCode == http.StatusOK {
					payload := map[string]any{}
					if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
						resp.Body.Close()
						reportErr(fmt.Errorf("json decode failed: %w", err))
						return
					}
				} else {
					if _, err := io.Copy(io.Discard, resp.Body); err != nil {
						resp.Body.Close()
						reportErr(fmt.Errorf("response read failed: %w", err))
						return
					}
				}

				if err := resp.Body.Close(); err != nil {
					reportErr(fmt.Errorf("response close failed: %w", err))
					return
				}
			}
		}(g)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case err := <-errCh:
			t.Fatal(err)
		default:
		}
	case err := <-errCh:
		t.Fatal(err)
	case <-testCtx.Done():
		t.Fatalf("scenario test timed out: %v", testCtx.Err())
	}
}
