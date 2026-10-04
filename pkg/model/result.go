package model

type Result struct {
	Target         Target
	ResolvedTarget string
	Process        Process
	RestartCount   int
	Ancestry       []Process
	Children       []Process `json:",omitempty"`
	Source         Source
	Warnings       []string

	// Container describes the container the process runs in (image, Compose
	// project and files), when the runtime can be queried.
	Container *ContainerMatch `json:",omitempty"`

	// SocketInfo holds socket state details (for port queries)
	SocketInfo *SocketInfo

	// ResourceContext holds resource usage context (macOS)
	ResourceContext *ResourceContext

	// FileContext holds file descriptor and lock info
	FileContext *FileContext
}
