package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"atad-project/internal/version"
)

// The marking script reads these fields. If this test fails, the contract is
// broken and the build is not gradeable.
func TestVersionReportsContractFields(t *testing.T) {
	var b strings.Builder
	if err := version.WriteJSON(&b); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	for _, field := range []string{`"app"`, `"version"`, `"commit"`, `"built_at"`} {
		if !strings.Contains(b.String(), field) {
			t.Errorf("version output is missing %s: %s", field, b.String())
		}
	}
}

// Replace this with tests for your own logic. Aim for the behaviour that would
// break silently, not for the lines that are easy to reach.
func TestYourLogic(t *testing.T) {
	t.Skip("write your own tests here")
func TestCreateLinkEndpoint(t *testing.T) {
	t.Run("creates link with supplied code", func(t *testing.T) {
		response := postCreateLink(newHandler(), `{"url":" https://example.com/path ","code":" custom1 "}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body)
		}
		if got := response.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var link Link
		if err := json.Unmarshal(response.Body.Bytes(), &link); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if link.ID != 1 {
			t.Errorf("id = %d, want 1", link.ID)
		}
		if link.Code != "custom1" {
			t.Errorf("code = %q, want custom1", link.Code)
		}
		if link.URL != "https://example.com/path" {
			t.Errorf("url = %q, want trimmed URL", link.URL)
		}
		if link.CreatedAt.IsZero() {
			t.Error("created_at is zero")
		}
	})

	t.Run("generates code when omitted", func(t *testing.T) {
		response := postCreateLink(newHandler(), `{"url":"https://example.com"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body)
		}

		var link Link
		if err := json.Unmarshal(response.Body.Bytes(), &link); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(link.Code) != 6 {
			t.Fatalf("generated code length = %d, want 6", len(link.Code))
		}
		for _, char := range link.Code {
			if !strings.ContainsRune(codeChars, char) {
				t.Errorf("generated code contains invalid character %q", char)
			}
		}
	})
	t.Run("rejects duplicate code", func(t *testing.T) {
		handler := newHandler()
		body := `{"url":"https://example.com","code":"taken"}`
		first := postCreateLink(handler, body)
		if first.Code != http.StatusCreated {
			t.Fatalf("first status = %d, want %d: %s", first.Code, http.StatusCreated, first.Body)
		}

		response := postCreateLink(handler, body)
		if response.Code != http.StatusConflict {
			t.Fatalf("duplicate status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body)
		}
		var got struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode error response: %v", err)
		}
		if got.Error != "conflict" || got.Message != "code already in use" {
			t.Errorf("error response = %+v, want conflict with duplicate-code message", got)
		}
	})
}

func postCreateLink(handler http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/links", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
