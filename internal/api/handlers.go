package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rah-0/lunar/internal/models"
	"github.com/rah-0/lunar/internal/storage"
)

// Handler contains the dependencies needed for the API handlers
type Handler struct {
	Repository storage.RocketRepository
}


func NewHandler(repo storage.RocketRepository) *Handler {
	return &Handler{Repository: repo}
}

// RegisterRoutes registers all API routes with the provided http.ServeMux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Root path redirects to Swagger
	mux.HandleFunc("GET /", h.HandleRoot)

	// POST endpoint to receive rocket messages
	mux.HandleFunc("POST /messages", h.HandleMessages)

	// GET endpoint to retrieve a specific rocket by ID
	mux.HandleFunc("GET /rockets/{id}", h.HandleGetRocket)

	// GET endpoint to list all rockets
	mux.HandleFunc("GET /rockets", h.HandleListRockets)

	// Health check endpoint
	mux.HandleFunc("GET /health", h.HandleHealth)

	// Serve Swagger documentation
	mux.HandleFunc("GET /swagger", h.HandleSwagger)
	mux.HandleFunc("GET /swagger/{path...}", h.HandleSwaggerAssets)
}

// @Summary Process a rocket message
// @Description Process a rocket message envelope
// @Tags messages
// @Accept json
// @Produce json
// @Param message body models.Envelope true "Message envelope"
// @Success 202 {object} map[string]any "Message accepted"
// @Failure 400 {object} map[string]any "Bad request"
// @Router /messages [post]
func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	// Parse the incoming JSON message
	var envelope models.Envelope
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Strict mode to catch malformed JSON

	if err := decoder.Decode(&envelope); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	// Validate the message
	if err := validateEnvelope(envelope); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid message format: "+err.Error())
		return
	}

	// Process the message
	processed := h.Repository.ProcessMessage(envelope)

	// Respond with success status
	response := map[string]any{
		"processed":     processed,
		"channel":       envelope.GetChannel(),
		"messageNumber": envelope.GetMessageNumber(),
	}

	respondWithJSON(w, http.StatusAccepted, response)
}

// HandleGetRocket retrieves a specific rocket by ID
// @Summary Get rocket by ID
// @Description Retrieve the complete rocket object including all its properties
// @Tags Rockets
// @Produce json
// @Param id path string true "Rocket ID"
// @Success 200 {object} models.RocketState "Complete rocket object"
// @Failure 404 {object} map[string]any "Rocket not found"
// @Router /rockets/{id} [get]
func (h *Handler) HandleGetRocket(w http.ResponseWriter, r *http.Request) {
	// Extract the rocket ID from the path parameter
	rocketID := r.PathValue("id")

	if rocketID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing rocket ID")
		return
	}

	// Find the rocket
	rocket, exists := h.Repository.GetRocket(rocketID)
	if !exists {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Rocket with ID %s not found", rocketID))
		return
	}

	// Return the rocket state
	respondWithJSON(w, http.StatusOK, rocket)
}

// HandleListRockets handles the GET /rockets endpoint
// @Summary List all rockets
// @Description Get a list of all rockets, optionally sorted by specified field and order
// @Tags rockets
// @Produce json
// @Param sort query string false "Sort field (e.g., 'id', 'speed', 'type', 'mission', 'status')"
// @Param order query string false "Sort order ('asc' or 'desc')"
// @Success 200 {array} models.RocketSummary "List of rocket summaries"
// @Router /rockets [get]
func (h *Handler) HandleListRockets(w http.ResponseWriter, r *http.Request) {
	// Get the sort and order parameters
	sortField := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")

	// Get the list of rockets with sort options
	rockets := h.Repository.ListRockets(sortField, order)

	// Return the rocket list
	respondWithJSON(w, http.StatusOK, rockets)
}

// HandleSwagger serves the Swagger UI index
func (h *Handler) HandleSwagger(w http.ResponseWriter, r *http.Request) {
	// Redirect to swagger/ (with trailing slash) to ensure relative paths resolve correctly
	if r.URL.Path == "/swagger" {
		http.Redirect(w, r, "/swagger/", http.StatusFound)
		return
	}

	http.ServeFile(w, r, "./docs/swagger/index.html")
}

// HandleSwaggerAssets serves static Swagger assets
func (h *Handler) HandleSwaggerAssets(w http.ResponseWriter, r *http.Request) {
	filePath := r.PathValue("path")

	// If it's the root of /swagger/ directory, serve index.html
	if filePath == "" {
		http.ServeFile(w, r, "./docs/swagger/index.html")
		return
	}

	// Serve the Swagger UI assets
	http.ServeFile(w, r, "./docs/swagger/"+filePath)
}

// HandleRoot redirects to the Swagger UI
func (h *Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	// Redirect to Swagger UI
	http.Redirect(w, r, "/swagger/", http.StatusFound)
}

// HandleHealth handles the GET /health endpoint for healthcheck
// @Summary Health check
// @Description Returns 200 OK when the service is healthy
// @Tags system
// @Produce json
// @Success 200 {object} map[string]string "Service status"
// @Router /health [get]
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	// Return simple health status
	respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// Helper functions for HTTP responses

// respondWithError sends an error response
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON sends a JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
