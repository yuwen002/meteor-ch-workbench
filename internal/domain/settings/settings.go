// Package settings 定义工作台用户偏好及其校验规则。
package settings

const (
	ThemeLight = "light"
	ThemeDark = "dark"
	RowLimitUnlimited = 0
	RowLimitMin = 100
	RowLimitMax = 1000000
	DefaultRowLimit = 10000
	DefaultEditorRatio = 0.45
	MinEditorRatio = 0.35
	MaxEditorRatio = 0.80
)

// Preferences 是可持久化的工作台偏好；JSON 字段兼容旧 settings.json。
type Preferences struct {
	Theme string `json:"theme"`
	QueryRowLimit int `json:"query_row_limit"`
	SidebarCollapsed bool `json:"sidebar_collapsed"`
	EditorRatio float64 `json:"editor_ratio"`
}

func Default() Preferences {
	return Preferences{Theme: ThemeLight, QueryRowLimit: DefaultRowLimit, EditorRatio: DefaultEditorRatio}
}

func Normalize(value Preferences) Preferences {
	if value.Theme != ThemeLight && value.Theme != ThemeDark { value.Theme = ThemeLight }
	value.QueryRowLimit = NormalizeRowLimit(value.QueryRowLimit)
	if value.EditorRatio < MinEditorRatio || value.EditorRatio > MaxEditorRatio {
		value.EditorRatio = DefaultEditorRatio
	}
	return value
}

func NormalizeRowLimit(value int) int {
	if value == RowLimitUnlimited { return RowLimitUnlimited }
	if value < RowLimitMin { return DefaultRowLimit }
	if value > RowLimitMax { return RowLimitMax }
	return value
}
