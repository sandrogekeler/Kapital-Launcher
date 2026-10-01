//go:build !windows && !darwin

package splashhost

import (
	"errors"
	"testing"
)

func TestWhereThereIsNoCardWindowOpenSaysSoAndTheRestIsSafe(t *testing.T) {
	h := New()
	if err := h.Open(Rect{W: 1, H: 1}, Page{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("got %v", err)
	}
	h.Update([]byte(`{}`))
	h.Close()
	h.Close()
}
