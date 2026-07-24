package ui

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2/widget"
)

func TestCustomExtensionsEmptyFieldReturnsNil(t *testing.T) {
	tab := &startTab{otherExtensions: widget.NewEntry()}
	if got := tab.customExtensions(); got != nil {
		t.Errorf("customExtensions() on an empty field = %v, want nil", got)
	}
}

func TestCustomExtensionsNilFieldReturnsNil(t *testing.T) {
	tab := &startTab{}
	if got := tab.customExtensions(); got != nil {
		t.Errorf("customExtensions() with no field built yet = %v, want nil", got)
	}
}

// TestCustomExtensionsAcceptsCommaOrPipeAndOptionalDot guards the actual
// point of this field: someone who doesn't want to write regex should be
// able to use either separator and either put a leading dot on each
// extension or not, interchangeably, and get the same clean list either
// way.
func TestCustomExtensionsAcceptsCommaOrPipeAndOptionalDot(t *testing.T) {
	cases := []string{
		"xls,doc,odt",
		"xls, doc, odt",
		"xls|doc|odt",
		".xls|.doc|.odt",
		".xls, .doc, odt",
		"  xls  ,  doc , odt ",
	}
	want := []string{"xls", "doc", "odt"}
	for _, in := range cases {
		tab := &startTab{otherExtensions: widget.NewEntry()}
		tab.otherExtensions.Text = in
		if got := tab.customExtensions(); !reflect.DeepEqual(got, want) {
			t.Errorf("customExtensions(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCustomExtensionsSkipsEmptyEntries(t *testing.T) {
	tab := &startTab{otherExtensions: widget.NewEntry()}
	tab.otherExtensions.Text = "xls,,doc| |odt"
	want := []string{"xls", "doc", "odt"}
	if got := tab.customExtensions(); !reflect.DeepEqual(got, want) {
		t.Errorf("customExtensions with blank entries = %v, want %v", got, want)
	}
}
