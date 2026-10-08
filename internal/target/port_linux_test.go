package target

import (
	"errors"
	"testing"
)

// A hidden owner is still named by the user its socket belongs to.
func TestOwnerUnknownError(t *testing.T) {
	if err := ownerUnknownError(map[string]int{"1": -1}); err != ErrSocketOwnerUnknown {
		t.Errorf("no known owner: %v, want the plain error", err)
	}
	err := ownerUnknownError(map[string]int{"1": 0, "2": 0, "3": 987654321})
	if !errors.Is(err, ErrSocketOwnerUnknown) {
		t.Fatalf("%v doesn't wrap ErrSocketOwnerUnknown", err)
	}
	if want := "socket found but owning process not detected; it belongs to 987654321, root"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}
