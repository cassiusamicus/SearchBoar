package ui

import (
	"codeberg.org/cassiusamicus/Utilities/internal/config"
	"codeberg.org/cassiusamicus/Utilities/internal/fsutil"
)

// openConfigFile opens config.ini in the user's default text editor, same
// as the original app's Options toolbar button. Changes only take effect
// on the next launch, since the config is loaded once at startup.
func (a *App) openConfigFile() {
	fsutil.OpenPath(config.DefaultPath())
}
