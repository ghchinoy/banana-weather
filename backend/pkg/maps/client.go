package maps

import (
	"context"
	"fmt"
	"log"

	"googlemaps.github.io/maps"
)

type Service struct {
	client *maps.Client
}

func NewService(apiKey string) (*Service, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_MAPS_API_KEY is empty")
	}

	c, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &Service{client: c}, nil
}

func (s *Service) GetReverseGeocoding(ctx context.Context, lat, lng float64) (string, error) {
	log.Printf("Reverse geocoding lat: %f, lng: %f", lat, lng)
	r, err := s.client.Geocode(ctx, &maps.GeocodingRequest{
		LatLng: &maps.LatLng{Lat: lat, Lng: lng},
	})
	if err != nil {
		log.Printf("Reverse geocoding failed: %v", err)
		return "", err
	}
	if len(r) == 0 {
		return "", fmt.Errorf("location not found")
	}

	// Extract city and state from address components of the first result
	var city, state, country string
	for _, component := range r[0].AddressComponents {
		for _, t := range component.Types {
			switch t {
			case "locality":
				city = component.LongName
			case "administrative_area_level_1":
				state = component.ShortName // Use ShortName for state (e.g. CO)
			case "country":
				country = component.ShortName // Use ShortName for country (e.g. US)
			}
		}
	}

	// Construct friendly name
	var friendlyName string
	if city != "" {
		friendlyName = city
		if state != "" {
			friendlyName += ", " + state
		} else if country != "" {
			friendlyName += ", " + country
		}
	}

	// Fallback logic
	if friendlyName == "" {
		// Try to find a result with type 'locality' if components failed
		for _, result := range r {
			for _, t := range result.Types {
				if t == "locality" {
					friendlyName = result.FormattedAddress
					break
				}
			}
			if friendlyName != "" {
				break
			}
		}
	}
	
	if friendlyName == "" {
		friendlyName = r[0].FormattedAddress
	}
	
	log.Printf("Reverse geocoding success: %s", friendlyName)
	return friendlyName, nil
}

func (s *Service) GetCityLocation(ctx context.Context, city string) (string, float64, float64, error) {
	log.Printf("Geocoding city: %s", city)
	r, err := s.client.Geocode(ctx, &maps.GeocodingRequest{
		Address: city,
	})
	if err != nil {
		log.Printf("Geocoding failed: %v", err)
		return "", 0, 0, err
	}
	if len(r) == 0 {
		log.Printf("Geocoding found no results for: %s", city)
		return "", 0, 0, fmt.Errorf("city not found")
	}

	formattedAddress := r[0].FormattedAddress
	lat := r[0].Geometry.Location.Lat
	lng := r[0].Geometry.Location.Lng
	
	log.Printf("Geocoding success: %s (Lat: %f, Lng: %f)", formattedAddress, lat, lng)

	return formattedAddress, lat, lng, nil
}
