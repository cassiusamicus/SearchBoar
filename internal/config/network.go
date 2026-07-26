package config

const sectionNetwork = "Network"

func (c *Config) loadNetwork(doc *iniDoc) {
	if v, ok := doc.get(sectionNetwork, "last_scan_cidr"); ok {
		c.LastScanCIDR = v
	}
}

func (c *Config) saveNetwork(doc *iniDoc) {
	if c.LastScanCIDR == "" {
		return
	}
	doc.section(sectionNetwork).set("last_scan_cidr", c.LastScanCIDR)
}
