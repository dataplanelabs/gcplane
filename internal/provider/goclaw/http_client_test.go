package goclaw

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassifyStatus(t *testing.T) {
	tests := []struct {
		code    int
		wantErr error
	}{
		{200, nil},
		{201, nil},
		{204, nil},
		{400, ErrInvalidRequest},
		{401, ErrUnauthorized},
		{403, ErrUnauthorized},
		{404, ErrNotFound},
		{422, ErrInvalidRequest},
		{500, ErrServerError},
		{503, ErrServerError},
	}

	for _, tc := range tests {
		err := classifyStatus(tc.code, []byte("body"))
		if tc.wantErr == nil {
			if err != nil {
				t.Errorf("status %d: expected nil, got %v", tc.code, err)
			}
		} else {
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("status %d: expected %v, got %v", tc.code, tc.wantErr, err)
			}
		}
	}
}

func TestClassifyStatus_UnexpectedCode(t *testing.T) {
	err := classifyStatus(301, []byte("moved"))
	if err == nil {
		t.Fatal("expected error for unexpected status 301")
	}
}

func TestHTTPClient_Get_AuthHeader(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "my-token")
	_, err := c.Get(context.Background(), "/v1/test")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if gotAuth != "Bearer my-token" {
		t.Errorf("expected Bearer my-token, got %q", gotAuth)
	}
}

func TestHTTPClient_Patch(t *testing.T) {
	var method string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok")
	_, err := c.Patch(context.Background(), "/v1/test", map[string]any{"key": "val"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if method != http.MethodPatch {
		t.Errorf("expected PATCH, got %s", method)
	}
}

func TestHTTPClient_Delete_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok")
	err := c.Delete(context.Background(), "/v1/missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestHTTPClient_ExtraHeaders(t *testing.T) {
	var gotUserID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = r.Header.Get("X-GoClaw-User-Id")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok")
	_, err := c.Get(context.Background(), "/v1/test")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if gotUserID != "gcplane" {
		t.Errorf("expected X-GoClaw-User-Id=gcplane, got %q", gotUserID)
	}
}
