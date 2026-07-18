package config

import "strconv"

const sectionColumns = "Columns"

func (c *Config) loadColumns(doc *iniDoc) {
	for _, key := range doc.keys(sectionColumns) {
		v, _ := doc.get(sectionColumns, key)
		if w, err := strconv.Atoi(v); err == nil && w > 0 {
			c.Columns[key] = w
		}
	}
}

func (c *Config) saveColumns(doc *iniDoc) {
	sec := doc.section(sectionColumns)
	for _, key := range sortedKeys(c.Columns) {
		sec.set(key, strconv.Itoa(c.Columns[key]))
	}
}
