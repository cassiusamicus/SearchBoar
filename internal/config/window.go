package config

import "strconv"

const sectionWindow = "Window"

func (c *Config) loadWindow(doc *iniDoc) {
	if v, ok := doc.get(sectionWindow, "x"); ok {
		c.Window.X = atoiOr(v, -1)
	}
	if v, ok := doc.get(sectionWindow, "y"); ok {
		c.Window.Y = atoiOr(v, -1)
	}
	if v, ok := doc.get(sectionWindow, "width"); ok {
		c.Window.Width = atoiOr(v, 0)
	}
	if v, ok := doc.get(sectionWindow, "height"); ok {
		c.Window.Height = atoiOr(v, 0)
	}
}

func (c *Config) saveWindow(doc *iniDoc) {
	sec := doc.section(sectionWindow)
	sec.set("x", strconv.Itoa(c.Window.X))
	sec.set("y", strconv.Itoa(c.Window.Y))
	sec.set("width", strconv.Itoa(c.Window.Width))
	sec.set("height", strconv.Itoa(c.Window.Height))
}

// HasPosition reports whether a saved window position should be applied,
// matching the original app's "only restore position if x >= 0 and y >= 0"
// rule (a missing/unset position defaults to -1).
func (w WindowState) HasPosition() bool {
	return w.X >= 0 && w.Y >= 0
}
