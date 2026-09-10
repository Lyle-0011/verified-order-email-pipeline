package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
)

type service struct {
	pipeline *orderPipeline
	mu       sync.Mutex
	orders   map[string]signup
}

func main() {
	key := requiredEnv("INFRAI_API_KEY")
	secret := requiredEnv("VERIFICATION_SECRET")
	publicURL := requiredEnv("PUBLIC_URL")
	svc := &service{
		pipeline: &orderPipeline{email: newInfraiClient(key), secret: []byte(secret), public: publicURL},
		orders:   make(map[string]signup),
	}
	http.HandleFunc("POST /signup", svc.handleSignup)
	http.HandleFunc("GET /verify", svc.handleVerify)
	log.Printf("verified order pipeline listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func (s *service) handleSignup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		OrderID string `json:"order_id"`
		Email   string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.OrderID == "" || input.Email == "" {
		http.Error(w, "order_id and email are required", http.StatusBadRequest)
		return
	}
	order, err := s.pipeline.start(r.Context(), input.OrderID, input.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.mu.Lock()
	s.orders[input.OrderID] = order
	s.mu.Unlock()
	writeJSON(w, http.StatusAccepted, order)
}

func (s *service) handleVerify(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	s.mu.Lock()
	current, found := s.orders[orderID]
	s.mu.Unlock()
	if !found || current.Email != r.URL.Query().Get("email") {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}
	updated, err := s.pipeline.verify(r.Context(), current, r.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.orders[orderID] = updated
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, updated)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
