package models

type GameInfo struct {
	AppID      uint32 `json:"appId"`
	Name       string `json:"name"`
	LogoURL    string `json:"logoUrl"`
	Installed  bool   `json:"installed"`
	LastPlayed uint32 `json:"lastPlayed"`
	IsSoftware bool   `json:"isSoftware"`
}
