package storage

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"

	"cloud.google.com/go/storage"
)

type Service struct {
	client     *storage.Client
	bucketName string
	projectID  string
}

func NewService(ctx context.Context, bucketName string) (*Service, error) {
	if bucketName == "" {
		return nil, fmt.Errorf("GENMEDIA_BUCKET is empty")
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	return &Service{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// ReadObject reads the content of a file from GCS.
func (s *Service) ReadObject(ctx context.Context, fileName string) ([]byte, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(fileName)
	
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

// UploadImage uploads a base64 image to GCS and returns (gsURI, publicURL).
func (s *Service) UploadImage(ctx context.Context, imageBase64 string, fileName string) (string, string, error) {
	data, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		return "", "", fmt.Errorf("invalid base64: %w", err)
	}
	// Reuse UploadBytes logic? 
	// Let's keep it distinct for now or refactor.
	// To avoid duplication, let's just call UploadBytes.
	// But UploadBytes returns one URL. UploadImage returns TWO (gsURI for Veo, Public for Frontend).
	// We need gsURI for Veo.
	
	// Inline implementation for Image (returns GS URI)
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(fileName)
	
	w := obj.NewWriter(ctx)
	w.ContentType = "image/png"
	if _, err := w.Write(data); err != nil {
		return "", "", fmt.Errorf("failed to write to bucket: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close writer: %w", err)
	}

	gsURI := fmt.Sprintf("gs://%s/%s", s.bucketName, fileName)
	publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", s.bucketName, fileName)

	log.Printf("Uploaded %s to %s", fileName, gsURI)
	return gsURI, publicURL, nil
}

// UploadBytes uploads raw bytes to GCS and returns the public URL.
func (s *Service) UploadBytes(ctx context.Context, data []byte, fileName string, mimeType string) (string, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(fileName)
	
	w := obj.NewWriter(ctx)
	w.ContentType = mimeType
	if _, err := w.Write(data); err != nil {
		return "", fmt.Errorf("failed to write to bucket: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", s.bucketName, fileName)
	log.Printf("Uploaded %d bytes to %s", len(data), publicURL)
	return publicURL, nil
}
