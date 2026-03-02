package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	db          *DB
	store       *sessions.CookieStore
	oauthConfig *oauth2.Config
)

func main() {
	sessionSecret := getEnv("SESSION_SECRET", "dev-secret-change-in-production")
	dbPath := getEnv("DATABASE_PATH", "tempus.db")
	port := getEnv("PORT", "8080")

	oauthConfig = &oauth2.Config{
		ClientID:     requireEnv("GOOGLE_CLIENT_ID"),
		ClientSecret: requireEnv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	store = sessions.NewCookieStore([]byte(sessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	var err error
	db, err = initDB(dbPath)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer db.close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Public
	r.Get("/login", handleLogin)
	r.Get("/auth/google", handleGoogleAuth)
	r.Get("/auth/callback", handleGoogleCallback)
	r.Get("/logout", handleLogout)

	// Protected
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/", handleIndex)
		r.Get("/day/{date}", handleDay)
		r.Post("/day/{date}/save", handleSaveDay)
		r.Get("/export/week", handleExportWeek)
	})

	log.Printf("Server running at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %q is not set", key)
	}
	return v
}
