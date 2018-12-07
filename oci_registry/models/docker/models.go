package docker

type Manifest struct {
	MediaType     string    `json:"mediaType"`
	SchemaVersion int       `json:"schemaVersion"`
	Config        Content   `json:"config"`
	Layers        []Content `json:"layers"`
}

type Content struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}
