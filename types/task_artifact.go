package types

// TaskArtifact is the transport-neutral identity of one generated task output.
// relay/channel re-exports this type for adaptor compatibility.
type TaskArtifact struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	MimeType string `json:"mimeType,omitempty"`
	// Name is the artifact's display filename when the plugin knows one. The
	// key is a constrained identifier (non-ASCII collapses to dashes), so the
	// dashboard needs this to show "报告.pptx" instead of "pptx" and to match
	// file paths the agent writes into its reply.
	Name string `json:"name,omitempty"`
}
