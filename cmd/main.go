package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"tempus/internal/auth"
	"tempus/internal/db"
	"tempus/internal/handlers"
)

func main() {
	sessionSecret := getEnv("SESSION_SECRET", "dev-secret-change-in-production")
	dbPath := getEnv("DATABASE_PATH", "tempus.db")
	port := getEnv("PORT", "8080")

	oauthConfig := &oauth2.Config{
		ClientID:     requireEnv("GOOGLE_CLIENT_ID"),
		ClientSecret: requireEnv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	store := sessions.NewCookieStore([]byte(sessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer database.Close()

	tmpl := template.Must(template.ParseGlob("templates/*.html"))

	authH := &auth.Handler{
		DB:     database,
		Store:  store,
		Config: oauthConfig,
		Tmpl:   tmpl,
	}
	pageH := &handlers.Handler{
		DB:   database,
		Tmpl: tmpl,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Health check
	r.Get("/up", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// Public
	r.Get("/login", authH.Login)
	r.Get("/auth/google", authH.GoogleAuth)
	r.Get("/auth/callback", authH.GoogleCallback)
	r.Get("/logout", authH.Logout)

	// Protected
	r.Group(func(r chi.Router) {
		r.Use(authH.Middleware)
		r.Get("/", pageH.Index)
		r.Get("/day", pageH.Index)
		r.Get("/day/{date}", pageH.Day)
		r.Post("/day/{date}/save", pageH.SaveDay)
		r.Get("/week/{date}", pageH.Week)
		r.Get("/export/week", pageH.ExportWeek)
		r.Post("/account/delete", authH.DeleteAccount)
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
