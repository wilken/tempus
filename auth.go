package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
)

const sessionName = "tempus-session"

type contextKey string

const (
	ctxUserID   contextKey = "user_id"
	ctxUserName contextKey = "user_name"
)

func handleLogin(w http.ResponseWriter, r *http.Request) {
	sess, _ := store.Get(r, sessionName)
	if id, ok := sess.Values["user_id"].(string); ok && id != "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	renderTemplate(w, "login.html", nil)
}

func handleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	sess, _ := store.Get(r, sessionName)
	sess.Values["oauth_state"] = state
	sess.Save(r, w)
	http.Redirect(w, r, oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	sess, _ := store.Get(r, sessionName)

	if r.URL.Query().Get("state") != sess.Values["oauth_state"] {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	delete(sess.Values, "oauth_state")

	token, err := oauthConfig.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	info, err := fetchGoogleUserInfo(r.Context(), token)
	if err != nil {
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	name, _ := info["name"].(string)
	if name == "" {
		name, _ = info["email"].(string)
	}
	user := User{
		ID:    info["sub"].(string),
		Email: info["email"].(string),
		Name:  name,
	}

	if err := db.upsertUser(user); err != nil {
		http.Error(w, "failed to save user", http.StatusInternalServerError)
		return
	}

	sess.Values["user_id"] = user.ID
	sess.Values["user_name"] = user.Name
	sess.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, _ := store.Get(r, sessionName)
	sess.Options.MaxAge = -1
	sess.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := store.Get(r, sessionName)
		userID, ok := sess.Values["user_id"].(string)
		if !ok || userID == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, userID)
		ctx = context.WithValue(ctx, ctxUserName, sess.Values["user_name"])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func fetchGoogleUserInfo(ctx context.Context, token *oauth2.Token) (map[string]interface{}, error) {
	resp, err := oauthConfig.Client(ctx, token).Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var info map[string]interface{}
	return info, json.NewDecoder(resp.Body).Decode(&info)
}

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
