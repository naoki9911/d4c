package update

type Image struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type DiffData struct {
	ImageName   string `json:"imageName"`
	FileName    string `json:"fileName"`
	ConfigPath  string `json:"configPath"`
	Version     string `json:"version"`
	BaseVersion string `json:"baseVersion"`
}

type UpdateDataRequest struct {
	RequestImage Image   `json:"requestImage"`
	LocalImages  []Image `json:"localImages"`
}

type UpdateDataResponse struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	BaseVersion string `json:"baseVersion"`
}
