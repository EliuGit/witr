package proc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The forwarder check matches on the executable name.
func TestImageNameOfSelf(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	// Linux keeps only the first 15 characters of the name.
	if got := imageName(os.Getpid()); got == "" || !strings.HasPrefix(filepath.Base(self), got) {
		t.Errorf("imageName(self) = %q, want a prefix of %q", got, filepath.Base(self))
	}
}

// An ordinary listener never makes a port a container's.
func TestPublishedContainerIgnoresOrdinaryListeners(t *testing.T) {
	if match, _ := PublishedContainer(80, []int{os.Getpid()}); match != nil {
		t.Errorf("PublishedContainer with a plain process = %+v, want nil", match)
	}
	if match, _ := PublishedContainer(80, nil); match != nil {
		t.Errorf("PublishedContainer with no listeners = %+v, want nil", match)
	}
}
