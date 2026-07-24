package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
	"codeberg.org/cassiusamicus/Utilities/internal/favorites"
	"codeberg.org/cassiusamicus/Utilities/internal/fsutil"
)

type favoritesTab struct {
	app  *App
	tree *widget.Tree

	categories []favorites.Category

	selectedPath     string
	selectedCategory string
}

func newFavoritesTab(a *App) *favoritesTab {
	return &favoritesTab{app: a}
}

func (t *favoritesTab) build() fyne.CanvasObject {
	t.categories = t.app.favStore.Categories()

	t.tree = widget.NewTree(t.childUIDs, t.isBranch,
		func(branch bool) fyne.CanvasObject { return newTappableBox(widget.NewLabel("")) },
		func(uid widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
			t.updateNode(uid, branch, o.(*tappableBox))
		},
	)
	t.tree.OnSelected = func(uid widget.TreeNodeID) {
		ci, fi, ok := decodeUID(uid)
		t.selectedPath = ""
		t.selectedCategory = ""
		if !ok || ci < 0 || ci >= len(t.categories) {
			return
		}
		if fi < 0 {
			t.selectedCategory = t.categories[ci].Name
		} else if fi < len(t.categories[ci].Items) {
			t.selectedPath = t.categories[ci].Items[fi].Filepath
			t.selectedCategory = t.categories[ci].Name
		}
	}

	buttons := container.NewHBox(
		widget.NewButton("Add Category", t.promptAddCategory),
		widget.NewButton("Rename Category", t.promptRenameCategory),
		widget.NewButton("Move to Category", t.promptMoveToCategory),
		widget.NewButton("Delete Category", t.promptDeleteCategory),
		widget.NewSeparator(),
		widget.NewButton("Move Up", func() { t.move(-1) }),
		widget.NewButton("Move Down", func() { t.move(1) }),
	)

	return container.NewBorder(buttons, nil, nil, nil, t.tree)
}

// refresh reloads categories from the store and redraws the tree; called
// after any favorite/category mutation (including from other tabs' "Add to
// Favorites" context menu action).
func (t *favoritesTab) refresh() {
	t.categories = t.app.favStore.Categories()
	if t.tree != nil {
		t.tree.Refresh()
	}
}

// uid scheme: "" is the root. "cat:<i>" is category i. "fav:<i>:<j>" is
// item j within category i. Rebuilt fresh from t.categories on every
// refresh, so indices are always valid for the current render.
func encodeCatUID(i int) string    { return fmt.Sprintf("cat:%d", i) }
func encodeFavUID(i, j int) string { return fmt.Sprintf("fav:%d:%d", i, j) }
func decodeUID(uid string) (ci, fi int, ok bool) {
	if uid == "" {
		return -1, -1, false
	}
	fi = -1
	if _, err := fmt.Sscanf(uid, "cat:%d", &ci); err == nil {
		return ci, -1, true
	}
	if _, err := fmt.Sscanf(uid, "fav:%d:%d", &ci, &fi); err == nil {
		return ci, fi, true
	}
	return -1, -1, false
}

func (t *favoritesTab) childUIDs(uid widget.TreeNodeID) []widget.TreeNodeID {
	if uid == "" {
		out := make([]widget.TreeNodeID, len(t.categories))
		for i := range t.categories {
			out[i] = encodeCatUID(i)
		}
		return out
	}
	ci, fi, ok := decodeUID(uid)
	if !ok || fi >= 0 || ci < 0 || ci >= len(t.categories) {
		return nil
	}
	items := t.categories[ci].Items
	out := make([]widget.TreeNodeID, len(items))
	for j := range items {
		out[j] = encodeFavUID(ci, j)
	}
	return out
}

func (t *favoritesTab) isBranch(uid widget.TreeNodeID) bool {
	if uid == "" {
		return true
	}
	_, fi, ok := decodeUID(uid)
	return ok && fi < 0
}

func (t *favoritesTab) updateNode(uid widget.TreeNodeID, branch bool, box *tappableBox) {
	ci, fi, ok := decodeUID(uid)
	if !ok || ci < 0 || ci >= len(t.categories) {
		return
	}
	if fi < 0 {
		cat := t.categories[ci]
		label := widget.NewLabel(fmt.Sprintf("%s (%d)", cat.Name, len(cat.Items)))
		box.SetObjects([]fyne.CanvasObject{label})
		box.OnDoubleTapped = nil
		box.OnSecondaryTapped = nil
		return
	}
	if fi >= len(t.categories[ci].Items) {
		return
	}
	item := t.categories[ci].Items[fi]
	text := item.Filename
	if item.SearchTerm != "" {
		text += "  —  " + item.SearchTerm
	}
	text += "  —  " + item.Modified + "  —  " + item.SizeHuman
	label := widget.NewLabel(text)

	// Same Open/Actions buttons as the Result Preview cards (see
	// resultsview.go's buildCard), not just the double-click-to-open and
	// right-click-for-menu this row already had -- those work but don't
	// show themselves, so there was previously no visible way to discover
	// either from just looking at the tree.
	openBtn := widget.NewButtonWithIcon("Open", theme.MediaPlayIcon(), func() {
		if err := t.app.openResult(item.Filepath); err != nil {
			t.app.setStatus("Failed to open: " + err.Error())
		}
	})
	openBtn.Importance = widget.LowImportance
	actionsBtn := widget.NewButtonWithIcon("Actions", theme.MoreVerticalIcon(), nil)
	actionsBtn.Importance = widget.LowImportance
	actionsBtn.OnTapped = func() {
		menu := t.favoriteContextMenu(item)
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(actionsBtn)
		pos = pos.Add(fyne.NewPos(0, actionsBtn.Size().Height))
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), pos)
	}

	// A spacer, not a Border-wrapped row: tappableBox's own content is a
	// plain HBox (every child laid out at its natural MinSize -- see
	// layout.NewHBoxLayout's doc comment), which would leave a
	// Border-wrapped row sized to its own minimum and the buttons sitting
	// right after the label instead of at the row's true right edge.
	// HBoxLayout gives a Spacer child special treatment, expanding it to
	// absorb whatever width Tree gives this row beyond the label/buttons'
	// own minimum -- the same mechanism resultsview.go's navHeader uses to
	// push its count label to the right.
	box.SetObjects([]fyne.CanvasObject{label, layout.NewSpacer(), openBtn, actionsBtn})
	box.OnDoubleTapped = func() {
		if err := t.app.openResult(item.Filepath); err != nil {
			t.app.setStatus("Failed to open: " + err.Error())
		}
	}
	box.OnSecondaryTapped = func(e *fyne.PointEvent) {
		menu := t.favoriteContextMenu(item)
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), e.AbsolutePosition)
	}
}

func (t *favoritesTab) favoriteContextMenu(item config.FavoriteRecord) *fyne.Menu {
	return fyne.NewMenu("",
		fyne.NewMenuItem("Open in default program", func() {
			if err := fsutil.OpenPath(item.Filepath); err != nil {
				t.app.setStatus("Failed to open: " + err.Error())
			}
		}),
		fyne.NewMenuItem("Open with...", func() { t.app.showOpenWithDialog(item.Filepath) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Show in file manager", func() {
			if err := fsutil.ShowInFileManager(item.Filepath); err != nil {
				t.app.setStatus("Failed to open file manager: " + err.Error())
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Copy path", func() {
			t.app.win.Clipboard().SetContent(item.Filepath)
			t.app.setStatus("Path copied to clipboard")
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Remove from Favorites", func() {
			t.app.favStore.Remove(item.Filepath)
			t.app.favStore.Save()
			t.refresh()
		}),
	)
}

func (t *favoritesTab) move(dir int) {
	if t.selectedPath == "" {
		return
	}
	var err error
	if dir < 0 {
		err = t.app.favStore.MoveUp(t.selectedPath)
	} else {
		err = t.app.favStore.MoveDown(t.selectedPath)
	}
	if err != nil {
		t.app.setStatus(err.Error())
		return
	}
	t.app.favStore.Save()
	t.refresh()
}

func (t *favoritesTab) promptAddCategory() {
	dialog.ShowEntryDialog("Add Category", "Category name:", func(name string) {
		if name == "" {
			return
		}
		if err := t.app.favStore.AddCategory(name); err != nil {
			t.app.setStatus(err.Error())
			return
		}
		t.app.favStore.Save()
		t.refresh()
	}, t.app.win)
}

func (t *favoritesTab) promptRenameCategory() {
	if t.selectedCategory == "" {
		t.app.setStatus("Select a category to rename")
		return
	}
	old := t.selectedCategory
	dialog.ShowEntryDialog("Rename Category", "New name for \""+old+"\":", func(name string) {
		if name == "" {
			return
		}
		if err := t.app.favStore.RenameCategory(old, name); err != nil {
			t.app.setStatus(err.Error())
			return
		}
		t.app.favStore.Save()
		t.refresh()
	}, t.app.win)
}

func (t *favoritesTab) promptMoveToCategory() {
	if t.selectedPath == "" {
		t.app.setStatus("Select a favorite to move")
		return
	}
	names := make([]string, len(t.categories))
	for i, c := range t.categories {
		names[i] = c.Name
	}
	path := t.selectedPath
	sel := widget.NewSelect(names, nil)
	dialog.ShowCustomConfirm("Move to Category", "Move", "Cancel", sel, func(ok bool) {
		if !ok || sel.Selected == "" {
			return
		}
		if err := t.app.favStore.MoveToCategory(path, sel.Selected); err != nil {
			t.app.setStatus(err.Error())
			return
		}
		t.app.favStore.Save()
		t.refresh()
	}, t.app.win)
}

func (t *favoritesTab) promptDeleteCategory() {
	if t.selectedCategory == "" {
		t.app.setStatus("Select a category to delete")
		return
	}
	name := t.selectedCategory
	dialog.ShowConfirm("Delete Category",
		fmt.Sprintf("Delete category %q? Files inside will be moved to Uncategorized.", name),
		func(ok bool) {
			if !ok {
				return
			}
			if err := t.app.favStore.DeleteCategory(name); err != nil {
				t.app.setStatus(err.Error())
				return
			}
			t.app.favStore.Save()
			t.refresh()
		}, t.app.win)
}
