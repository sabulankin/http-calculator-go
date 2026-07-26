package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEval(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want float64
	}{
		{name: "precedence", expr: "2+3*4", want: 14},
		{name: "parentheses", expr: "(3+5)*2", want: 16},
		{name: "unary minus", expr: "-2*4", want: -8},
		{name: "zero", expr: "1-1", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eval(tt.expr)
			if err != nil {
				t.Fatalf("eval(%q) returned error: %v", tt.expr, err)
			}
			if got != tt.want {
				t.Fatalf("eval(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestEvalErrors(t *testing.T) {
	for _, expr := range []string{"1/0", "(1+2", "1+a", "1+"} {
		t.Run(expr, func(t *testing.T) {
			if _, err := eval(expr); err == nil {
				t.Fatalf("eval(%q) unexpectedly succeeded", expr)
			}
		})
	}
}

func TestResultsHandlerValidatesDateRangeBeforeDatabaseAccess(t *testing.T) {
	application := &app{}
	tests := []string{
		"/results",
		"/results?from=bad&to=2026-01-02T00:00:00Z",
		"/results?from=2026-01-03T00:00:00Z&to=2026-01-02T00:00:00Z",
	}

	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, target, nil)
			response := httptest.NewRecorder()

			application.resultsHandler(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandlersRejectUnsupportedMethods(t *testing.T) {
	application := &app{}
	tests := []struct {
		handler http.HandlerFunc
		target  string
		method  string
		allow   string
	}{
		{handler: application.calculateHandler, target: "/calc", method: http.MethodGet, allow: http.MethodPost},
		{handler: application.resultsHandler, target: "/results", method: http.MethodPost, allow: http.MethodGet},
	}

	for _, tt := range tests {
		request := httptest.NewRequest(tt.method, tt.target, nil)
		response := httptest.NewRecorder()

		tt.handler(response, request)

		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s status = %d, want %d", tt.method, tt.target, response.Code, http.StatusMethodNotAllowed)
		}
		if got := response.Header().Get("Allow"); got != tt.allow {
			t.Fatalf("Allow = %q, want %q", got, tt.allow)
		}
	}
}
