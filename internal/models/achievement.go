package models

// Achievement represents a game achievement with its current state.
type Achievement struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	IconURL     string  `json:"iconUrl"`
	IconGrayURL string  `json:"iconGrayUrl"`
	IsAchieved  bool    `json:"isAchieved"`
	UnlockTime  uint32  `json:"unlockTime"`
	IsHidden    bool    `json:"isHidden"`
	Permission  int     `json:"permission"`
	Percent     float32 `json:"percent"`
}
