package config

const sectionCustomPrograms = "CustomPrograms"

func (c *Config) loadCustomPrograms(doc *iniDoc) {
	for _, name := range doc.keys(sectionCustomPrograms) {
		v, _ := doc.get(sectionCustomPrograms, name)
		c.CustomPrograms[name] = v
	}
}

func (c *Config) saveCustomPrograms(doc *iniDoc) {
	sec := doc.section(sectionCustomPrograms)
	for _, name := range sortedKeys(c.CustomPrograms) {
		sec.set(name, c.CustomPrograms[name])
	}
}
