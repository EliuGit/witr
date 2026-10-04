package proc

import (
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

func TestParseLabelString(t *testing.T) {
	got := parseLabelString("com.docker.compose.project=web, com.docker.compose.service=api")
	if got["com.docker.compose.project"] != "web" {
		t.Errorf("project = %q, want web", got["com.docker.compose.project"])
	}
	if got["com.docker.compose.service"] != "api" {
		t.Errorf("service = %q, want api", got["com.docker.compose.service"])
	}
	if len(parseLabelString("")) != 0 {
		t.Error("empty input should yield an empty map")
	}
}

func TestHealthFromStatus(t *testing.T) {
	tests := map[string]string{
		"Up 4 minutes (healthy)":         "healthy",
		"Up 2 seconds (unhealthy)":       "unhealthy",
		"Up 1 second (health: starting)": "starting",
		"Up 5 minutes":                   "", // no health check wired
		"Exited (0) 3 minutes ago":       "", // parens not at end of status
	}
	for in, want := range tests {
		if got := healthFromStatus(in); got != want {
			t.Errorf("healthFromStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseDockerTime(t *testing.T) {
	if !parseDockerTime("").IsZero() {
		t.Error("empty input should yield the zero time")
	}
	if !parseDockerTime("not a timestamp").IsZero() {
		t.Error("garbage input should yield the zero time")
	}
	got := parseDockerTime("2024-01-02T15:04:05Z")
	if got.IsZero() || got.Year() != 2024 {
		t.Errorf("parseDockerTime(RFC3339) = %v, want a 2024 time", got)
	}
}

// Docker and Podman inspect documents carry the start time, restart count and
// policy; nerdctl may leave the policy out without losing the rest.
func TestApplyDockerInspect(t *testing.T) {
	tests := []struct {
		name, doc   string
		wantCount   int
		wantPolicy  string
		wantStarted bool
	}{
		{"docker", `{"State":{"StartedAt":"2026-10-04T09:04:31.123456789Z"},"RestartCount":3,"HostConfig":{"RestartPolicy":{"Name":"unless-stopped","MaximumRetryCount":0}}}`, 3, "unless-stopped", true},
		{"on-failure limit", `{"State":{"StartedAt":"2026-10-04T09:04:31Z"},"RestartCount":2,"HostConfig":{"RestartPolicy":{"Name":"on-failure","MaximumRetryCount":5}}}`, 2, "on-failure:5", true},
		{"no policy field", `{"State":{"StartedAt":"2026-10-04T09:04:31Z"},"RestartCount":1}`, 1, "", true},
		{"never started", `{"State":{"StartedAt":"0001-01-01T00:00:00Z"},"RestartCount":0,"HostConfig":{"RestartPolicy":{"Name":"no"}}}`, 0, "no", false},
	}
	for _, tt := range tests {
		m := &model.ContainerMatch{}
		applyDockerInspect(m, []byte(tt.doc))
		if m.RestartCount != tt.wantCount || m.RestartPolicy != tt.wantPolicy || m.StartedAt.IsZero() == tt.wantStarted {
			t.Errorf("%s: got count=%d policy=%q started=%v", tt.name, m.RestartCount, m.RestartPolicy, m.StartedAt)
		}
	}
	m := &model.ContainerMatch{RestartPolicy: "always"}
	applyDockerInspect(m, []byte("not json"))
	if m.RestartPolicy != "always" {
		t.Errorf("an unreadable document changed the match: %+v", m)
	}
}
