package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRecoveryMiddleware(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	mux.HandleFunc("/goroutine-panic", func(w http.ResponseWriter, r *http.Request) {
		var wg conc.WaitGroup
		wg.Go(func() {
			var m map[string]int
			m["x"] = 1
		})
		wg.Wait()
	})
	mux.HandleFunc("/partial", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		panic("boom")
	})
	mux.HandleFunc("/abort", func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(recoveryMiddleware(zap.NewNop(), mux))
	defer srv.Close()

	get := func(path string) (*http.Response, string, error) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return resp, string(body), err
	}

	for _, path := range []string{"/panic", "/goroutine-panic"} {
		t.Run(path, func(t *testing.T) {
			resp, body, err := get(path)
			require.NoError(t, err)
			require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
			require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			require.JSONEq(t, `{"Error":"internal server error"}`, body)
		})
	}
	t.Run("partial response is aborted", func(t *testing.T) {
		_, _, err := get("/partial")
		require.Error(t, err)
		require.True(t, errors.Is(err, io.ErrUnexpectedEOF), err)
	})
	t.Run("ErrAbortHandler is passed through", func(t *testing.T) {
		_, _, err := get("/abort")
		require.Error(t, err)
	})
	t.Run("server keeps serving", func(t *testing.T) {
		resp, body, err := get("/ok")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "ok", body)
	})
}
