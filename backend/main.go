package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"banana-weather/api"
	"banana-weather/pkg/config"
	"banana-weather/pkg/database"
	"banana-weather/pkg/genai"
	"banana-weather/pkg/maps"
	"banana-weather/pkg/storage"
	"banana-weather/pkg/weather"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err)
	}

	// Initialize Services
	mapsService, err := maps.NewService(cfg.GoogleMapsKey)
	if err != nil {
		log.Fatalf("FATAL: Maps service failed to initialize. Error: %v", err)
	}

	// GenAI Service
	genaiService, err := genai.NewService(context.Background(), cfg.ProjectID, cfg.ImageLocation, cfg.VideoLocation, cfg.BucketName, cfg.GeminiImageModel, cfg.VeoVideoModel)
	if err != nil {
		log.Fatalf("FATAL: GenAI service failed to initialize. Error: %v", err)
	}

	// Storage Service
	storageService, err := storage.NewService(context.Background(), cfg.BucketName)
	if err != nil {
		log.Printf("Warning: Storage service failed to initialize: %v", err)
	}

	// Database Service
	dbService, err := database.NewClient(context.Background(), cfg.ProjectID, cfg.DatabaseID)
	if err != nil {
		log.Fatalf("FATAL: Database service failed to initialize. Error: %v", err)
	}
	defer dbService.Close()

	// Weather Orchestrator
	weatherService := weather.NewService(mapsService, genaiService, storageService, dbService)

	handler := &api.Handler{
		DB:      dbService,
		Weather: weatherService,
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

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout is intentionally 0 because /api/weather streams SSE over
		// Veo video generation (~1-2 minutes).
		IdleTimeout: 120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown error: %v", err)
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
