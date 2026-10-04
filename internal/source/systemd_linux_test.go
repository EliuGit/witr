//go:build linux

package source

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/pranshuparmar/witr/pkg/model"
)

func TestCalendarSpec(t *testing.T) {
	// D-Bus delivers TimersCalendar as [][]interface{}{{base, spec, next}}.
	v := [][]interface{}{{"OnCalendar", "*-*-* 06,18:00:00", uint64(123)}}
	if got := calendarSpec(v); got != "*-*-* 06,18:00:00" {
		t.Errorf("calendarSpec = %q, want the calendar expression", got)
	}
	if got := calendarSpec(nil); got != "" {
		t.Errorf("calendarSpec(nil) = %q, want empty", got)
	}
}

func TestMonotonicSpec(t *testing.T) {
	day := uint64((24 * time.Hour) / time.Microsecond)
	tests := []struct {
		base string
		want string
	}{
		{"OnUnitActiveUSec", "every 1d"},
		{"OnBootUSec", "every boot + 1d"},
		{"OnUnitInactiveUSec", "every 1d after idle"},
	}
	for _, tt := range tests {
		v := [][]interface{}{{tt.base, day, uint64(0)}}
		if got := monotonicSpec(v); got != tt.want {
			t.Errorf("monotonicSpec(%s) = %q, want %q", tt.base, got, tt.want)
		}
	}
	if got := monotonicSpec(nil); got != "" {
		t.Errorf("monotonicSpec(nil) = %q, want empty", got)
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		25 * time.Hour:   "1d 1h",
		24 * time.Hour:   "1d",
		90 * time.Minute: "1h 30min",
		90 * time.Second: "1min 30s",
		5 * time.Second:  "5s",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestUsecToTime(t *testing.T) {
	if !usecToTime(0).IsZero() {
		t.Error("usecToTime(0) should be the zero time")
	}
	if !usecToTime(math.MaxUint64).IsZero() {
		t.Error("usecToTime(MaxUint64) should be the zero time (systemd 'n/a' sentinel)")
	}
	got := usecToTime(uint64(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC).UnixMicro()))
	if got.IsZero() || got.UTC().Year() != 2024 {
		t.Errorf("usecToTime(real value) = %v, want a 2024 time", got)
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()
	// Durations are buffered off the truncation boundaries so sub-second drift
	// between now and the time.Since() call inside the function can't flip an
	// "N min" bucket to "N-1".
	cases := []struct {
		d    time.Duration // offset from now; negative = past, positive = future
		want string
	}{
		{-30 * time.Second, "<1 min ago"},
		{-(5*time.Minute + 30*time.Second), "5 min ago"},
		{-(3*time.Hour + 30*time.Minute), "3h ago"},
		{-50 * time.Hour, "2d ago"},
		{30 * time.Second, "in <1 min"},
		{5*time.Minute + 30*time.Second, "in 5 min"},
		{3*time.Hour + 30*time.Minute, "in 3h"},
		{50 * time.Hour, "in 2d"},
	}
	for _, tc := range cases {
		if got := formatRelativeTime(now.Add(tc.d)); got != tc.want {
			t.Errorf("formatRelativeTime(now%+v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestGetUnitNameFromCgroupSelf(t *testing.T) {
	// The result depends on the host's cgroup layout (a .scope, a .service, or
	// nothing), so we only exercise the read+parse without asserting a value.
	_ = getUnitNameFromCgroup(os.Getpid())

	// PID 0 has no cgroup file, so the read fails and we get "".
	if got := getUnitNameFromCgroup(0); got != "" {
		t.Errorf("getUnitNameFromCgroup(0) = %q, want empty", got)
	}
}

func TestApplyUnitProperties(t *testing.T) {
	tests := []struct {
		name               string
		unit               map[string]interface{}
		wantDesc, wantFile string
	}{
		{"description and unit file", map[string]interface{}{"Description": "A high performance web server", "FragmentPath": "/lib/systemd/system/nginx.service", "SourcePath": "/etc/init.d/nginx"}, "A high performance web server", "/lib/systemd/system/nginx.service"},
		{"generated from an init script", map[string]interface{}{"Description": "LSB: web server", "SourcePath": "/etc/init.d/nginx"}, "LSB: web server", "/etc/init.d/nginx"},
		{"no description of its own", map[string]interface{}{"Description": "nginx.service"}, "", ""},
		{"unexpected types", map[string]interface{}{"Description": 7, "FragmentPath": []byte("x")}, "", ""},
	}
	for _, tt := range tests {
		src := &model.Source{}
		applyUnitProperties(src, "nginx.service", tt.unit)
		if src.Description != tt.wantDesc || src.UnitFile != tt.wantFile {
			t.Errorf("%s: description %q, unit file %q; want %q, %q", tt.name, src.Description, src.UnitFile, tt.wantDesc, tt.wantFile)
		}
	}
}

func TestRestartCount(t *testing.T) {
	if got := restartCount(map[string]interface{}{"NRestarts": uint32(7)}); got != "7" {
		t.Errorf("restartCount = %q, want 7", got)
	}
	if got := restartCount(map[string]interface{}{}); got != "0" {
		t.Errorf("restartCount without the property = %q, want 0", got)
	}
}
