package main

import "testing"

func TestNextStage(t *testing.T) {
	tests := []struct {
		name       string
		current    orderStage
		tokenValid bool
		want       orderStage
	}{
		{name: "verified signup enters checkout", current: stageAwaitingEmail, tokenValid: true, want: stageCheckoutReady},
		{name: "invalid token holds signup", current: stageAwaitingEmail, tokenValid: false, want: stageAwaitingEmail},
		{name: "replay does not rewind fulfillment", current: stageFulfillment, tokenValid: true, want: stageFulfillment},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextStage(tt.current, tt.tokenValid); got != tt.want {
				t.Fatalf("nextStage(%q, %v) = %q, want %q", tt.current, tt.tokenValid, got, tt.want)
			}
		})
	}
}
