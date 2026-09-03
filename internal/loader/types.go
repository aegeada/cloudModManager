package loader

type FabricVersion struct {
	Separator string `json:"separator"`
	Build     int    `json:"build"`
	Maven     string `json:"maven"`
	Version   string `json:"version"`
	Stable    bool   `json:"stable"`
}

type LoaderInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Supported   bool   `json:"supported"`
}
