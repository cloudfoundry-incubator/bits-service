package docker

type Versioned struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType,omitempty"`
}
type Manifest struct {
	Versioned
	Config Content   `json:"config"`
	Layers []Content `json:"layers"`
}

type Content struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ManifestList struct {
	Versioned
	Manifests []ManifestDescriptor `json:"manifests"`
}

type ManifestDescriptor struct {
	Content
	Platform PlatformSpec `json:"platform"`
}

type PlatformSpec struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"`
}
