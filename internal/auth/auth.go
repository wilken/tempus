package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"

	"tempus/internal/db"
)

const sessionName = "tempus-session"

// ContextKey is the type for context keys set by auth middleware.
type ContextKey string

const (
	CtxUserID   ContextKey = "user_id"
	CtxUserName ContextKey = "user_name"
)

// Handler holds auth dependencies and exposes HTTP handlers and middleware.
type Handler struct {
	DB     *db.DB
	Store  *sessions.CookieStore
	Config *oauth2.Config
	Tmpl   *template.Template
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	sess, _ := h.Store.Get(r, sessionName)
	if id, ok := sess.Values["user_id"].(string); ok && id != "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	h.renderTemplate(w, "login.html", nil)
}

func (h *Handler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	sess, _ := h.Store.Get(r, sessionName)
	sess.Values["oauth_state"] = state
	sess.Save(r, w)
	http.Redirect(w, r, h.Config.AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusTemporaryRedirect)
}

func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	sess, _ := h.Store.Get(r, sessionName)

	if r.URL.Query().Get("state") != sess.Values["oauth_state"] {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	delete(sess.Values, "oauth_state")

	token, err := h.Config.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	info, err := h.fetchGoogleUserInfo(r.Context(), token)
	if err != nil {
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	name, _ := info["name"].(string)
	if name == "" {
		name, _ = info["email"].(string)
	}
	user := db.User{
		ID:    info["sub"].(string),
		Email: info["email"].(string),
		Name:  name,
	}

	if err := h.DB.UpsertUser(user); err != nil {
		http.Error(w, "failed to save user", http.StatusInternalServerError)
		return
	}

	sess.Values["user_id"] = user.ID
	sess.Values["user_name"] = user.Name
	sess.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sess, _ := h.Store.Get(r, sessionName)
	sess.Options.MaxAge = -1
	sess.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := h.Store.Get(r, sessionName)
		userID, ok := sess.Values["user_id"].(string)
		if !ok || userID == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), CtxUserID, userID)
		ctx = context.WithValue(ctx, CtxUserName, sess.Values["user_name"])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) fetchGoogleUserInfo(ctx context.Context, token *oauth2.Token) (map[string]interface{}, error) {
	resp, err := h.Config.Client(ctx, token).Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var info map[string]interface{}
	return info, json.NewDecoder(resp.Body).Decode(&info)
}

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
