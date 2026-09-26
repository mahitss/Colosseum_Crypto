package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

type copilotQueryRequest struct {
	Message         string `json:"message"`
	ConversationID  string `json:"conversation_id,omitempty"`
}

type copilotSource struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Title     string `json:"title,omitempty"`
	MarketID  string `json:"market_id,omitempty"`
	Source    string `json:"source"`
}

type copilotQueryResponse struct {
	Answer      string          `json:"answer"`
	Sources     []copilotSource `json:"sources"`
	Intent      string          `json:"intent"`
	ToolCalls   []interface{}   `json:"tool_calls"`
	GeneratedAt string          `json:"generated_at"`
}

type copilotErrorResponse struct {
	Error string `json:"error"`
}

func registerCopilotRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/copilot/query", handleCopilotQuery)
	mux.HandleFunc("GET /api/v1/copilot/health", handleCopilotHealth)
}

func handleCopilotQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(copilotErrorResponse{Error: "Method not allowed"})
		return
	}

	var req copilotQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(copilotErrorResponse{Error: "Invalid request body"})
		return
	}

	if strings.TrimSpace(req.Message) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(copilotErrorResponse{Error: "Message is required"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(copilotQueryResponse{
		Answer:      "Prophet Copilot is now connected to the real backend. In production, this would query the Python intelligence service for grounded market intelligence.",
		Sources:     []copilotSource{},
		Intent:      "GENERAL_QUERY",
		GeneratedAt: "2026-09-26T19:35:00Z",
	})
}

func handleCopilotHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}
