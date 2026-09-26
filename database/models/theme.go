package models

// Theme represents the embedded Lite theme manifest.
type Theme struct {
	Name          any           `json:"name"`
	Short         string        `json:"short"`
	Description   any           `json:"description"`
	Version       string        `json:"version"`
	Author        any           `json:"author"`
	URL           string        `json:"url"`
	Preview       string        `json:"preview"`
	Configuration Configuration `json:"configuration"`
}

type Configuration struct {
	Type string `json:"type"` // managed raw redirect
	Icon string `json:"icon"` // 图标
	Name any    `json:"name"`
	Data any    `json:"data"` // 配置数据
}

type ManagedThemeConfigurationItem struct {
	Key      string `json:"key"`
	Name     any    `json:"name"`
	Required bool   `json:"required"`
	Type     string `json:"type"` // string number select switch title textbox richtext nodes pingtasks
	Options  string `json:"options"`
	Default  any    `json:"default"`
	Help     any    `json:"help"`
}

type ThemeConfiguration struct {
	Short string `json:"short" gorm:"primaryKey;unique;not null"`
	Data  string `json:"data" gorm:"type:longtext" default:"{}"`
}
