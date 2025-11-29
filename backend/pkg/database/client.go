package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type Client struct {
	fs *firestore.Client
}

func NewClient(ctx context.Context) (*Client, error) {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = os.Getenv("PROJECT_ID")
	}
	
	databaseID := os.Getenv("FIRESTORE_DATABASE")
	if databaseID == "" {
		// Default to standard DB if not set, but we prefer explicit
		databaseID = "(default)"
	}

	log.Printf("Initializing Firestore. Project: %s, Database: %s", projectID, databaseID)

	// Create client with specific database ID
	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}

	return &Client{fs: client}, nil
}

// Close closes the Firestore client.
func (c *Client) Close() error {
	return c.fs.Close()
}

// -- Models --

type Location struct {
	ID          string    `firestore:"id" json:"id"`
	Name        string    `firestore:"name" json:"name"`         // Display Name
	Category    string    `firestore:"category" json:"category"` // Grouping
	CityQuery   string    `firestore:"city_query" json:"city_query"` // Original input
	ImageURL    string    `firestore:"image_url" json:"image_url"`
	VideoURL    string    `firestore:"video_url" json:"video_url"`
	IsPreset    bool      `firestore:"is_preset" json:"is_preset"` // Admin managed?
	LastUpdated time.Time `firestore:"last_updated" json:"last_updated"`
}

// -- Methods --

// GetPresets returns all locations where is_preset = true.
func (c *Client) GetPresets(ctx context.Context) ([]Location, error) {
	var presets []Location
	iter := c.fs.Collection("locations").Where("is_preset", "==", true).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var loc Location
		if err := doc.DataTo(&loc); err != nil {
			log.Printf("Failed to parse preset doc %s: %v", doc.Ref.ID, err)
			continue
		}
		presets = append(presets, loc)
	}
	return presets, nil
}

// UpsertLocation creates or updates a location document.
func (c *Client) UpsertLocation(ctx context.Context, loc Location) error {
	// Use ID as document ID if possible, ensuring uniqueness.
	// If ID is empty (new user search), maybe hash the city query?
	// For presets, ID is set.
	
	if loc.ID == "" {
		return fmt.Errorf("location ID is required")
	}

	loc.LastUpdated = time.Now()
	_, err := c.fs.Collection("locations").Doc(loc.ID).Set(ctx, loc)
	return err
}

// GetLocation retrieves a location by ID.
func (c *Client) GetLocation(ctx context.Context, id string) (*Location, error) {
	doc, err := c.fs.Collection("locations").Doc(id).Get(ctx)
	if err != nil {
		return nil, err // Returns NotFound status code if missing
	}
	var loc Location
	if err := doc.DataTo(&loc); err != nil {
		return nil, err
	}
	return &loc, nil
}
