package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type copilotQueryRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id,omitempty"`
}

type copilotSource struct {
	Type     string `json:"type"`
	ID       string `json:"id,omitempty"`
	Title    string `json:"title,omitempty"`
	MarketID string `json:"market_id,omitempty"`
	Source   string `json:"source"`
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

	// Forward to Python intelligence service
	intelligenceURL := os.Getenv("INTELLIGENCE_SERVICE_URL")
	if intelligenceURL == "" {
		intelligenceURL = "http://localhost:8001"
	}

	reqBody, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", intelligenceURL+"/v1/copilot/query", bytes.NewBuffer(reqBody))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(copilotErrorResponse{Error: "Failed to create request"})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(copilotErrorResponse{Error: "Intelligence service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func handleCopilotHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}
