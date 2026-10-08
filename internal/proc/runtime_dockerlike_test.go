package proc

import (
	"strings"
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
		d, ok := parseContainerInspect([]byte(tt.doc))
		if !ok {
			t.Fatalf("%s: parseContainerInspect failed", tt.name)
		}
		applyDockerInspect(m, d)
		if m.RestartCount != tt.wantCount || m.RestartPolicy != tt.wantPolicy || m.StartedAt.IsZero() == tt.wantStarted {
			t.Errorf("%s: got count=%d policy=%q started=%v", tt.name, m.RestartCount, m.RestartPolicy, m.StartedAt)
		}
	}
	if _, ok := parseContainerInspect([]byte("not json")); ok {
		t.Error("an unreadable document parsed")
	}
}

// A command may contain "|" (sh -c 'a | b'); the fields after it must not
// shift, or the state, ports and Compose labels come out wrong.
func TestParseDockerLikeList(t *testing.T) {
	line := func(fields ...string) string { return strings.Join(fields, listFieldSep) }
	out := line("5d9581a8eafb", "web-1", "nginx:stable-alpine", `"/docker-entrypoint.sh sh -c 'nginx -g \"daemon off;\" | cat'"`,
		"running", "Up 4 minutes (healthy)", "2026-10-04 13:54:18 +0000 UTC", "app_default", "/data", "0.0.0.0:18095->80/tcp, [::]:18095->80/tcp",
		"com.docker.compose.project=app,com.docker.compose.service=web,com.docker.compose.project.config_files=/srv/app/compose.yml,com.docker.compose.project.working_dir=/srv/app,note=a|b") +
		"\n\n" + line("abc", "short line") + "\n" +
		line("61c2dcb0795f", "witr-pd-slirp", "docker.io/library/nginx:latest", "nginx -g daemon off;", "running", "Up 2 minutes", "2026-10-04 13:34:30.1 +0000 UTC", "", "", "0.0.0.0:18200->80/tcp", "")

	got := parseDockerLikeList(out, "docker")
	if len(got) != 2 {
		t.Fatalf("parsed %d containers, want 2 (the short line skipped)", len(got))
	}
	want := model.ContainerMatch{
		Runtime: "docker", ID: "5d9581a8eafb", Name: "web-1", Image: "nginx:stable-alpine",
		Command: `/docker-entrypoint.sh sh -c 'nginx -g "daemon off;" | cat'`,
		State:   "running", Status: "Up 4 minutes (healthy)", Health: "healthy",
		CreatedAt: got[0].CreatedAt, Networks: "app_default", Mounts: "/data",
		Ports:          "0.0.0.0:18095->80/tcp, [::]:18095->80/tcp",
		ComposeProject: "app", ComposeService: "web",
		ComposeConfigFile: "/srv/app/compose.yml", ComposeWorkingDir: "/srv/app",
	}
	if *got[0] != want {
		t.Errorf("container with | in its command:\n got %+v\nwant %+v", *got[0], want)
	}
	if got[0].CreatedAt.IsZero() {
		t.Error("CreatedAt not parsed")
	}
	if m := got[1]; m.Name != "witr-pd-slirp" || m.Ports != "0.0.0.0:18200->80/tcp" || m.Health != "" || m.ComposeProject != "" {
		t.Errorf("second container = %+v", *m)
	}
}
