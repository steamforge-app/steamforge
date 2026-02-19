package schema

type AchievementDefinition struct {
	ID          string
	Name        string
	Description string
	IconNormal  string
	IconLocked  string
	IsHidden    bool
	Permission  int
}
