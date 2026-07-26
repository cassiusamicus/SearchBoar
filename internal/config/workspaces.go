package config

import "strings"

const sectionWorkspaces = "Workspaces"

// workspaceListSep joins the list-valued fields (local roots, exclude
// dirs, SMB shares, NFS exports) packed into one pipe-delimited value per
// workspace. It's a control character rather than a comma or pipe
// specifically so it can never collide with a real path, hostname, or
// share name.
const workspaceListSep = "\x1e"

func joinWorkspaceList(items []string) string { return strings.Join(items, workspaceListSep) }

func splitWorkspaceList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, workspaceListSep)
}

func boolFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// loadWorkspaces parses [Workspaces]; a value must have exactly 8
// pipe-delimited parts (search_local|search_smb|search_nfs|local_roots|
// exclude_dirs|smb_shares|nfs_exports|smb_share_excludes) to be considered
// valid. A workspace saved before smb_share_excludes existed has exactly 7
// parts and is still accepted, with that field simply left empty.
func (c *Config) loadWorkspaces(doc *iniDoc) {
	for _, name := range doc.keys(sectionWorkspaces) {
		v, _ := doc.get(sectionWorkspaces, name)
		parts := strings.SplitN(v, "|", 8)
		if len(parts) != 7 && len(parts) != 8 {
			continue
		}
		w := LocationWorkspace{
			Name:        name,
			SearchLocal: parts[0] == "1",
			SearchSMB:   parts[1] == "1",
			SearchNFS:   parts[2] == "1",
			LocalRoots:  splitWorkspaceList(parts[3]),
			ExcludeDirs: splitWorkspaceList(parts[4]),
			SMBShares:   splitWorkspaceList(parts[5]),
			NFSExports:  splitWorkspaceList(parts[6]),
		}
		if len(parts) == 8 {
			w.SMBShareExcludes = splitWorkspaceList(parts[7])
		}
		c.Workspaces[name] = w
	}
}

func (c *Config) saveWorkspaces(doc *iniDoc) {
	sec := doc.section(sectionWorkspaces)
	for _, name := range sortedKeys(c.Workspaces) {
		w := c.Workspaces[name]
		value := strings.Join([]string{
			boolFlag(w.SearchLocal), boolFlag(w.SearchSMB), boolFlag(w.SearchNFS),
			joinWorkspaceList(w.LocalRoots), joinWorkspaceList(w.ExcludeDirs),
			joinWorkspaceList(w.SMBShares), joinWorkspaceList(w.NFSExports),
			joinWorkspaceList(w.SMBShareExcludes),
		}, "|")
		sec.set(name, value)
	}
}
