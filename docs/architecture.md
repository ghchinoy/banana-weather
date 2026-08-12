# Banana Weather Architecture

## Overview

Banana Weather is a "GenMedia" application that visualizes the atmospheric essence of a location using Generative AI. It employs a **Client-Server** architecture where a Go backend orchestrates the generation process and serves a Flutter Web frontend.

## Components

### 1. The Portal (Frontend)
*   **Technology:** Flutter Web (Dart)
*   **Responsibility:**
    *   User Interface for displaying the generated artwork (9:16 aspect ratio).
    *   Input form for city selection.
    *   API Client communicating with the backend.
*   **Deployment:** Compiled to static HTML/JS/WASM and served by the Go backend.

### 2. The Temple (Backend)
*   **Technology:** Go 1.25+
*   **Responsibility:**
    *   **API Server:** Exposes `/api/weather` endpoint.
    *   **Static Host:** Serves the compiled Flutter application.
    *   **Geocoding:** Uses Google Maps API to resolve user input (e.g., "Paris") to a formatted address (e.g., "Paris, France") and coordinates.
    *   **GenAI Orchestrator:** Constructs the prompt and calls Vertex AI (Gemini 3.1 Flash Image / Nano Banana 2 by default, or Gemini 3 Pro Image / Nano Banana Pro) to generate the image, then Veo 3.1 to animate it.
*   **Deployment:** Containerized via Docker and deployed to Google Cloud Run.

## Data Flow

1.  **User** enters a city name in the Flutter UI.
2.  **Frontend** sends `GET /api/weather?city=Name` to the Backend.
3.  **Backend** calls **Google Maps Geocoding API** to validate and format the city name.
4.  **Backend** constructs a prompt using the current date and formatted city name.
5.  **Backend** calls **Vertex AI (Gemini)** to generate the image.
6.  **Backend** returns the image (Base64 encoded) and formatted city name to the Frontend.
7.  **Frontend** decodes and displays the image.

## Infrastructure

*   **Google Cloud Run:** Hosts the containerized application.
*   **Vertex AI:** Provides the Generative AI models.
*   **Google Maps Platform:** Provides location services.
