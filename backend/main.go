package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"banana-weather/api"
	"banana-weather/pkg/database"
	"banana-weather/pkg/genai"
	"banana-weather/pkg/maps"
	"banana-weather/pkg/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize Services
	mapsService, err := maps.NewService()
	if err != nil {
		log.Fatalf("FATAL: Maps service failed to initialize. Check GOOGLE_MAPS_API_KEY. Error: %v", err)
	}

	// GenAI Service
	genaiService, err := genai.NewService(context.Background())
	if err != nil {
		log.Fatalf("FATAL: GenAI service failed to initialize. Check PROJECT_ID/GOOGLE_CLOUD_PROJECT. Error: %v", err)
	}

	// Storage Service
	storageService, err := storage.NewService(context.Background())
	if err != nil {
		log.Printf("Warning: Storage service failed to initialize (Check GENMEDIA_BUCKET): %v", err)
	}

	// Database Service
	dbService, err := database.NewClient(context.Background())
	if err != nil {
		log.Fatalf("FATAL: Database service failed to initialize. Check FIRESTORE_DATABASE. Error: %v", err)
	}
	defer dbService.Close()

	handler := &api.Handler{
		Maps:    mapsService,
		GenAI:   genaiService,
		Storage: storageService,
		DB:      dbService,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/weather", handler.HandleGetWeather)
		r.Get("/presets", handler.HandleGetPresets)
	})

	// Static Files (Frontend)
	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, "../frontend/build/web")

	// Check if local path exists, otherwise assume Docker structure
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		// In Docker, we are in /app. Frontend is copied to /app/frontend/build/web
		// So relative path is just "frontend/build/web"
		filesDir = filepath.Join(workDir, "frontend/build/web")
	}

	log.Printf("Serving static files from: %s", filesDir)
	FileServer(r, "/", http.Dir(filesDir))

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}