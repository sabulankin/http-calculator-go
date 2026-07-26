package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		{name: "negative number", expr: "10*-2", want: -20},
		{name: "decimal", expr: "7.5/2.5", want: 3},
		{name: "zero result", expr: "1-1", want: 0},
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
	for _, expr := range []string{"1/0", "(1+2", "2+a", "1+"} {
		t.Run(expr, func(t *testing.T) {
			if _, err := eval(expr); err == nil {
				t.Fatalf("eval(%q) unexpectedly succeeded", expr)
			}
		})
	}
}

func TestCalculateHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/calc", strings.NewReader(`{"expr":"(3+5)*2"}`))
	recorder := httptest.NewRecorder()

	calculateHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response CalcResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Result != 16 {
		t.Fatalf("result = %v, want 16", response.Result)
	}
}

func TestCalculateHandlerRejectsInvalidRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/calc", strings.NewReader(`{"expr":"1/0"}`))
	recorder := httptest.NewRecorder()

	calculateHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
