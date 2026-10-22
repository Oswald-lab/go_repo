package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHello(t *testing.T) {
	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Hello from Go") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestUsersRequireAPIKey(t *testing.T) {
	t.Setenv("API_KEY", "secret")

	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCreateAndGetUser(t *testing.T) {
	t.Setenv("API_KEY", "secret")

	router := newRouter()
	createReq := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Ada","email":"ada@example.com","password":"s3cret"}`))
	createReq.Header.Set("X-API-Key", "secret")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", createRec.Code, createRec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected user id")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/users/"+created.ID, nil)
	getReq.Header.Set("X-API-Key", "secret")
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getRec.Code)
	}
	if !strings.Contains(getRec.Body.String(), "ada@example.com") {
		t.Fatalf("unexpected body: %s", getRec.Body.String())
	}
	if strings.Contains(getRec.Body.String(), "password") {
		t.Fatalf("response must not expose password data: %s", getRec.Body.String())
	}
}

func TestLoginWithValidCredentials(t *testing.T) {
	t.Setenv("API_KEY", "secret")

	router := newRouter()
	createReq := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Ada","email":"ada@example.com","password":"s3cret"}`))
	createReq.Header.Set("X-API-Key", "secret")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", createRec.Code, createRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"ada@example.com","password":"s3cret"}`))
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200: %s", loginRec.Code, loginRec.Body.String())
	}
	if !strings.Contains(loginRec.Body.String(), "ada@example.com") {
		t.Fatalf("unexpected body: %s", loginRec.Body.String())
	}
}

func TestLoginWithInvalidCredentials(t *testing.T) {
	t.Setenv("API_KEY", "secret")

	router := newRouter()
	createReq := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"Ada","email":"ada@example.com","password":"s3cret"}`))
	createReq.Header.Set("X-API-Key", "secret")
	router.ServeHTTP(httptest.NewRecorder(), createReq)

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"ada@example.com","password":"wrong"}`))
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want 401: %s", loginRec.Code, loginRec.Body.String())
	}
}

func TestLoginRequiresEmailAndPassword(t *testing.T) {
	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"","password":""}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
