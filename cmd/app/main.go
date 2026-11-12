// Entry point for your project.
//
// Every ATAD project is a web service in a container. The four routes below are
// the common contract and are checked automatically: keep them working exactly
// as specified and add your own routes alongside them.
//
// --version must also keep working. It prints one line of JSON and exits 0.
package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"atad-project/internal/version"
)

// started is the moment the process came up; /health reports the difference.
var started = time.Now()

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-v" {
			return version.WriteJSON(os.Stdout)
		}
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mux := http.NewServeMux()

	store := NewLinkStore()

	// ---- the common contract: four routes, identical in every project -------

	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = version.WriteJSON(w)
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":         "ok",
			"uptime_seconds": int(time.Since(started).Seconds()),
		})
	})

	// The identity page. Replace the placeholders with your own details; the
	// marking script reads this page.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8">
<title>%s</title><body>
<h1>%s</h1>
<p>[Surname Firstname], group [group]</p>
<p>Project [number]: [title]</p>
</body></html>`, version.Current().App, version.Current().App)
	})

	// Discards everything the service holds, so tests can start from a known
	// state without restarting the container. If your project persists data,
	// this clears what was persisted too.
	mux.HandleFunc("POST /reset", func(w http.ResponseWriter, r *http.Request) {
		store.Reset()
		w.WriteHeader(http.StatusNoContent)
	})

	// ---- your own routes go below -------------------------------------------

	mux.HandleFunc("POST /links", func(w http.ResponseWriter, r *http.Request) {
		var req CreateLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid json body")
			return
		}

		req.URL = strings.TrimSpace(req.URL)
		if req.URL == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "url is required")
			return
		}

		req.Code = strings.TrimSpace(req.Code)
		link, err := store.Create(req.URL, req.Code)
		if err != nil {
			if errors.Is(err, ErrCodeExists) {
				writeError(w, http.StatusConflict, "conflict", "code already in use")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, link)
	})

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Shut down cleanly on Ctrl-C or on the signal Docker sends when stopping
	// the container, so in-flight requests are allowed to finish.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "commit", version.Current().Commit)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// writeJSON is the one place that sets the content type, so every response in
// the service is consistent about it.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError produces the error shape the common contract requires. Use it for
// every failure rather than writing the JSON by hand at each call site.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

var ErrCodeExists = errors.New("code already exists")

type Link struct {
	ID        int       `json:"id"`
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateLinkRequest struct {
	URL  string `json:"url"`
	Code string `json:"code,omitempty"`
}

type LinkStore struct {
	mu     sync.RWMutex
	links  map[string]Link
	byID   map[int]Link
	nextID int
}

func NewLinkStore() *LinkStore {
	return &LinkStore{
		links:  make(map[string]Link),
		byID:   make(map[int]Link),
		nextID: 1,
	}
}

func (s *LinkStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.links = make(map[string]Link)
	s.byID = make(map[int]Link)
	s.nextID = 1
}

const codeChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = codeChars[int(b)%len(codeChars)]
	}
	return string(bytes), nil
}

func (s *LinkStore) Create(url, code string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if code != "" {
		if _, exists := s.links[code]; exists {
			return Link{}, ErrCodeExists
		}
	} else {
		for {
			var err error
			code, err = generateCode(6)
			if err != nil {
				return Link{}, err
			}
			if _, exists := s.links[code]; !exists {
				break
			}
		}
	}

	link := Link{
		ID:        s.nextID,
		Code:      code,
		URL:       url,
		CreatedAt: time.Now().UTC(),
	}
	s.nextID++
	s.links[code] = link
	s.byID[link.ID] = link

	return link, nil
}
