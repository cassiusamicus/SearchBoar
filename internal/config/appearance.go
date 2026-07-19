package config

const sectionAppearance = "Appearance"

func (c *Config) loadAppearance(doc *iniDoc) {
	if v, ok := doc.get(sectionAppearance, "accent_color"); ok {
		c.AccentColor = v
	}
	if v, ok := doc.get(sectionAppearance, "theme_mode"); ok {
		c.ThemeMode = v
	}
}

func (c *Config) saveAppearance(doc *iniDoc) {
	if c.AccentColor == "" && c.ThemeMode == "" {
		return
	}
	sec := doc.section(sectionAppearance)
	if c.AccentColor != "" {
		sec.set("accent_color", c.AccentColor)
	}
	if c.ThemeMode != "" {
		sec.set("theme_mode", c.ThemeMode)
	}
}
