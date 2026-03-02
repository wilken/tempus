package auth

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
)

func newTestStore() *sessions.CookieStore {
	return sessions.NewCookieStore([]byte("test-secret"))
}

// requestWithSession saves a session with the given values to a dummy recorder,
// then copies the resulting Set-Cookie header onto a fresh request.
func requestWithSession(t *testing.T, store *sessions.CookieStore, values map[interface{}]interface{}) *http.Request {
	t.Helper()

	dummy := httptest.NewRecorder()
	dummyReq := httptest.NewRequest("GET", "/", nil)
	sess, err := store.Get(dummyReq, sessionName)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	for k, v := range values {
		sess.Values[k] = v
	}
	if err := sess.Save(dummyReq, dummy); err != nil {
		t.Fatalf("save session: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range dummy.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestMiddlewareRedirectsUnauthenticated(t *testing.T) {
	store := newTestStore()
	h := &Handler{Store: store}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/", nil) // no session cookie
	rec := httptest.NewRecorder()
	h.Middleware(next).ServeHTTP(rec, req)

	if called {
		t.Error("next handler should not have been called")
	}
	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestMiddlewarePopulatesContext(t *testing.T) {
	store := newTestStore()
	h := &Handler{Store: store}

	var gotUserID, gotUserName string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, _ = r.Context().Value(CtxUserID).(string)
		gotUserName, _ = r.Context().Value(CtxUserName).(string)
		w.WriteHeader(http.StatusOK)
	})

	req := requestWithSession(t, store, map[interface{}]interface{}{
		"user_id":   "uid-123",
		"user_name": "Alice",
	})
	rec := httptest.NewRecorder()
	h.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "uid-123" {
		t.Errorf("expected user_id uid-123, got %q", gotUserID)
	}
	if gotUserName != "Alice" {
		t.Errorf("expected user_name Alice, got %q", gotUserName)
	}
}

func TestLoginRedirectsWhenAlreadyLoggedIn(t *testing.T) {
	store := newTestStore()
	h := &Handler{Store: store}

	req := requestWithSession(t, store, map[interface{}]interface{}{
		"user_id": "uid-123",
	})
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %q", loc)
	}
}

func TestLoginRendersPageWhenNotLoggedIn(t *testing.T) {
	store := newTestStore()
	tmpl := template.Must(template.New("login.html").Parse(`<html>login</html>`))
	h := &Handler{Store: store, Tmpl: tmpl}

	req := httptest.NewRequest("GET", "/login", nil) // no session
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content type, got %q", ct)
	}
}
