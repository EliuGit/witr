package model

import "time"

type ContainerMatch struct {
	Runtime           string
	ID                string
	Name              string
	Image             string
	Command           string
	State             string
	Status            string
	Health            string
	RestartCount      int    `json:",omitempty"` // restarts by the runtime (Kubernetes: the container's attempt number)
	RestartPolicy     string `json:",omitempty"` // e.g. always, unless-stopped, on-failure:5
	CreatedAt         time.Time
	StartedAt         time.Time
	Networks          string
	Mounts            string
	Ports             string
	ComposeProject    string `json:",omitempty"`
	ComposeService    string `json:",omitempty"`
	ComposeConfigFile string `json:",omitempty"`
	ComposeWorkingDir string `json:",omitempty"`
}
