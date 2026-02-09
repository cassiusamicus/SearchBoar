#!/usr/bin/env python3
"""
SearchBoar - GTK3 File Search Application
A Python/GTK3 implementation replicating SearchMonkey functionality
"""

import gi
gi.require_version('Gtk', '3.0')
gi.require_version('Gdk', '3.0')
from gi.repository import Gtk, GLib, Pango, Gdk, GdkPixbuf
import os
import re
import time
import threading
import json
import configparser
import subprocess
from pathlib import Path
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed

VERSION = "2.0"


class RegexBuilderDialog(Gtk.Dialog):
    """Dialog for building regular expressions graphically"""
    
    def __init__(self, parent, title="Regular Expression Builder"):
        super().__init__(title=title, transient_for=parent, flags=0)
        self.add_buttons(
            Gtk.STOCK_CANCEL, Gtk.ResponseType.CANCEL,
            Gtk.STOCK_OK, Gtk.ResponseType.OK
        )
        
        self.set_default_size(500, 400)
        
        box = self.get_content_area()
        
        # Pattern type selection
        type_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        type_box.set_margin_start(10)
        type_box.set_margin_end(10)
        type_box.set_margin_top(10)
        
        type_label = Gtk.Label(label="Pattern Type:")
        self.pattern_combo = Gtk.ComboBoxText()
        self.pattern_combo.append_text("Exact match")
        self.pattern_combo.append_text("Contains text")
        self.pattern_combo.append_text("Starts with")
        self.pattern_combo.append_text("Ends with")
        self.pattern_combo.append_text("File extension")
        self.pattern_combo.append_text("Email address")
        self.pattern_combo.append_text("IP address")
        self.pattern_combo.append_text("Custom regex")
        self.pattern_combo.set_active(1)
        self.pattern_combo.connect("changed", self.on_pattern_changed)
        
        type_box.pack_start(type_label, False, False, 0)
        type_box.pack_start(self.pattern_combo, True, True, 0)
        box.pack_start(type_box, False, False, 0)
        
        # Input text
        input_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        input_box.set_margin_start(10)
        input_box.set_margin_end(10)
        input_box.set_margin_top(10)
        
        input_label = Gtk.Label(label="Text:")
        self.input_entry = Gtk.Entry()
        self.input_entry.connect("changed", self.update_regex)
        
        input_box.pack_start(input_label, False, False, 0)
        input_box.pack_start(self.input_entry, True, True, 0)
        box.pack_start(input_box, False, False, 0)
        
        # Options
        options_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        options_box.set_margin_start(10)
        options_box.set_margin_end(10)
        options_box.set_margin_top(10)
        
        self.case_sensitive = Gtk.CheckButton(label="Case sensitive")
        self.case_sensitive.connect("toggled", self.update_regex)
        
        options_box.pack_start(self.case_sensitive, False, False, 0)
        box.pack_start(options_box, False, False, 0)
        
        # Generated regex display
        regex_frame = Gtk.Frame(label="Generated Regular Expression")
        regex_frame.set_margin_start(10)
        regex_frame.set_margin_end(10)
        regex_frame.set_margin_top(10)
        
        self.regex_view = Gtk.TextView()
        self.regex_view.set_editable(True)
        self.regex_view.set_wrap_mode(Gtk.WrapMode.CHAR)
        self.regex_buffer = self.regex_view.get_buffer()
        
        scrolled = Gtk.ScrolledWindow()
        scrolled.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        scrolled.add(self.regex_view)
        scrolled.set_size_request(-1, 100)
        
        regex_frame.add(scrolled)
        box.pack_start(regex_frame, True, True, 0)
        
        # Test section
        test_frame = Gtk.Frame(label="Test Pattern")
        test_frame.set_margin_start(10)
        test_frame.set_margin_end(10)
        test_frame.set_margin_top(10)
        test_frame.set_margin_bottom(10)
        
        test_vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        test_vbox.set_margin_start(6)
        test_vbox.set_margin_end(6)
        test_vbox.set_margin_top(6)
        test_vbox.set_margin_bottom(6)
        
        test_label = Gtk.Label(label="Test text:")
        test_vbox.pack_start(test_label, False, False, 0)
        
        self.test_entry = Gtk.Entry()
        self.test_entry.set_placeholder_text("Enter text to test against the pattern")
        self.test_entry.connect("changed", self.test_pattern)
        test_vbox.pack_start(self.test_entry, False, False, 0)
        
        self.test_result = Gtk.Label()
        test_vbox.pack_start(self.test_result, False, False, 0)
        
        test_frame.add(test_vbox)
        box.pack_start(test_frame, False, False, 0)
        
        self.show_all()
        self.update_regex()
    
    def on_pattern_changed(self, combo):
        """Handle pattern type change"""
        pattern_type = combo.get_active()
        # Enable/disable input based on pattern type
        self.input_entry.set_sensitive(pattern_type != 5 and pattern_type != 6)
        self.update_regex()
    
    def update_regex(self, widget=None):
        """Update the generated regex based on selections"""
        pattern_type = self.pattern_combo.get_active()
        text = self.input_entry.get_text()
        case_sensitive = self.case_sensitive.get_active()
        
        regex = ""
        
        if pattern_type == 0:  # Exact match
            regex = re.escape(text)
        elif pattern_type == 1:  # Contains
            regex = re.escape(text)
        elif pattern_type == 2:  # Starts with
            regex = f"^{re.escape(text)}"
        elif pattern_type == 3:  # Ends with
            regex = f"{re.escape(text)}$"
        elif pattern_type == 4:  # File extension
            regex = f"\\.{re.escape(text)}$"
        elif pattern_type == 5:  # Email
            regex = r"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"
        elif pattern_type == 6:  # IP address
            regex = r"\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b"
        elif pattern_type == 7:  # Custom
            regex = text
        
        if not case_sensitive and pattern_type != 7:
            regex = f"(?i){regex}"
        
        self.regex_buffer.set_text(regex)
        self.test_pattern()
    
    def test_pattern(self, widget=None):
        """Test the regex against test text"""
        test_text = self.test_entry.get_text()
        regex_text = self.regex_buffer.get_text(
            self.regex_buffer.get_start_iter(),
            self.regex_buffer.get_end_iter(),
            False
        )
        
        if not test_text or not regex_text:
            self.test_result.set_markup("")
            return
        
        try:
            pattern = re.compile(regex_text)
            matches = list(pattern.finditer(test_text))
            
            if matches:
                self.test_result.set_markup(
                    f'<span foreground="green">✓ Match found ({len(matches)} occurrence(s))</span>'
                )
            else:
                self.test_result.set_markup(
                    '<span foreground="red">✗ No match</span>'
                )
        except re.error as e:
            self.test_result.set_markup(
                f'<span foreground="red">✗ Invalid regex: {str(e)}</span>'
            )
    
    def get_regex(self):
        """Get the generated regex"""
        return self.regex_buffer.get_text(
            self.regex_buffer.get_start_iter(),
            self.regex_buffer.get_end_iter(),
            False
        )


class SearchBoar(Gtk.Window):
    """Main application window"""
    
    def __init__(self):
        super().__init__(title="SearchBoar")
        self.set_default_size(900, 600)
        self.set_border_width(10)
        
        # Print startup banner
        print("\n" + "="*70)
        print("  SearchBoar v1.0.9 - File Search Application")
        print("="*70)
        
        # Verbosity for terminal debugging (0=quiet, 1=normal, 2=debug)
        # Can be set via environment variable SEARCHBOAR_VERBOSE
        try:
            self.verbosity = int(os.environ.get("SEARCHBOAR_VERBOSE", "1"))
        except Exception:
            self.verbosity = 1

        self.search_thread = None
        self.stop_search = False
        self.has_ripgrep = False  # Will be set during dependency check
        
        # Config file setup
        self.config_dir = Path.home() / '.config' / 'searchboar'
        self.config_file = self.config_dir / 'config.ini'
        print(f"Config directory: {self.config_dir}")
        print(f"Config file: {self.config_file}")
        
        self.config = configparser.ConfigParser()
        self.config.optionxform = str  # Preserve case in keys (important for file paths!)
        self.load_config()
        
        # Setup the user interface
        print("Setting up UI...")
        self.setup_ui()
        
        # Load window state (position and size)
        self.load_window_state()
        
        # Load column widths after UI is ready
        GLib.idle_add(self.load_column_widths)
        
        # Check for all optional dependencies
        GLib.idle_add(self.check_dependencies)
        
        # Connect window close event to save settings
        self.connect("delete-event", self.on_window_close)
        
        print("SearchBoar initialized successfully!")
        print("="*70 + "\n")
    
    def log(self, level, msg):
        """Terminal logging helper controlled by SEARCHBOAR_VERBOSE env var."""
        try:
            verbosity = getattr(self, 'verbosity', 1)
        except Exception:
            verbosity = 1
        if verbosity >= level:
            ts = time.strftime("%Y-%m-%d %H:%M:%S")
            print(f"[{ts}] {msg}", flush=True)

    def _pattern_mentions_ext(self, pattern, ext):
        """Heuristic: does a filename regex/pattern appear to target an extension."""
        try:
            p = (pattern or "").lower()
        except Exception:
            p = ""
        e = (ext or "").lower().lstrip(".")
        if not e:
            return False
        # Handles both regex like \.pdf$ and simple substrings like .pdf
        return (f"\\.{e}" in p) or (f".{e}" in p)

    def check_dependencies(self):
        """Check for all optional dependencies and show install command if needed"""
        print("\nChecking optional dependencies...")
        missing = []
        
        # Check for ripgrep
        try:
            result = subprocess.run(['which', 'rg'], capture_output=True, text=True)
            if result.returncode != 0:
                missing.append('ripgrep')
                print("  ✗ ripgrep not found")
            else:
                self.has_ripgrep = True
                print("  ✓ ripgrep found")
        except:
            missing.append('ripgrep')
            self.has_ripgrep = False
            print("  ✗ ripgrep not found")
        
        # Check for pdftotext (from poppler package)
        try:
            result = subprocess.run(['which', 'pdftotext'], capture_output=True, text=True)
            if result.returncode != 0:
                missing.append('poppler')
                print("  ✗ pdftotext not found")
            else:
                print("  ✓ pdftotext found")
        except:
            missing.append('poppler')
            print("  ✗ pdftotext not found")
        
        # Check for PyPDF2 (Python library)
        try:
            import PyPDF2
            print("  ✓ PyPDF2 found")
        except ImportError:
            missing.append('python-pypdf2')
            print("  ✗ PyPDF2 not found")
        
        if not missing:
            print("All optional dependencies installed!")
        
        # If anything is missing, show install dialog
        if missing:
            self.show_dependency_install_dialog(missing)
    
    def show_dependency_install_dialog(self, missing):
        """Show dialog with yay command to install missing dependencies"""
        # Build the yay command (works for both official repos and AUR)
        yay_command = f"yay -S {' '.join(missing)}"
        
        # Create dialog
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.INFO,
            buttons=Gtk.ButtonsType.OK,
            text="Optional Dependencies Missing"
        )
        
        # Build description text
        desc_parts = ["SearchBoar works without these, but installing them provides:\n"]
        
        if 'ripgrep' in missing:
            desc_parts.append("• ripgrep: 10-100x faster content searches")
        if 'poppler' in missing:
            desc_parts.append("• poppler: PDF text extraction (fastest method)")
        if 'python-pypdf2' in missing:
            desc_parts.append("• python-pypdf2: PDF text extraction (Python library)")
        
        desc_parts.append("\n\nCopy and paste this command to install missing packages:\n")
        
        dialog.format_secondary_text('\n'.join(desc_parts))
        
        # Create content area with copyable command
        content_area = dialog.get_content_area()
        
        # Add frame for command
        frame = Gtk.Frame()
        frame.set_margin_start(10)
        frame.set_margin_end(10)
        frame.set_margin_top(5)
        frame.set_margin_bottom(10)
        
        # Command text in a text view for easy selection
        text_view = Gtk.TextView()
        text_view.set_editable(False)
        text_view.set_wrap_mode(Gtk.WrapMode.CHAR)
        text_view.set_left_margin(10)
        text_view.set_right_margin(10)
        text_view.set_top_margin(10)
        text_view.set_bottom_margin(10)
        
        # Use CSS for reliable styling across all themes
        css_provider = Gtk.CssProvider()
        css_provider.load_from_data(b"""
            textview {
                background-color: #1e1e1e;
                color: #ffffff;
                font-family: monospace;
                font-weight: bold;
                font-size: 11pt;
            }
            textview text {
                background-color: #1e1e1e;
                color: #ffffff;
            }
        """)
        
        style_context = text_view.get_style_context()
        style_context.add_provider(css_provider, Gtk.STYLE_PROVIDER_PRIORITY_USER)
        
        buffer = text_view.get_buffer()
        buffer.set_text(yay_command)
        
        # Make text selectable
        text_view.set_cursor_visible(True)
        
        # Add scrolled window (in case command is long)
        scrolled = Gtk.ScrolledWindow()
        scrolled.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.NEVER)
        scrolled.set_min_content_height(60)
        scrolled.add(text_view)
        
        frame.add(scrolled)
        content_area.pack_start(frame, False, False, 0)
        
        # Add helper label
        help_label = Gtk.Label()
        help_label.set_markup('<small><i>Tip: Text is pre-selected, just press Ctrl+C to copy</i></small>')
        help_label.set_margin_start(10)
        help_label.set_margin_end(10)
        help_label.set_margin_bottom(10)
        content_area.pack_start(help_label, False, False, 0)
        
        dialog.show_all()
        
        # Auto-select the command text
        buffer.select_range(buffer.get_start_iter(), buffer.get_end_iter())
        text_view.grab_focus()
        
        dialog.run()
        dialog.destroy()
    
    def is_binary_file(self, filepath):
        """Check if file is binary by looking for null bytes"""
        try:
            with open(filepath, 'rb') as f:
                chunk = f.read(8192)
                return b'\0' in chunk
        except:
            return True
    
    def extract_pdf_text(self, filepath):
        """Extract text from PDF file"""
        self.log(1, f"DEBUG: Attempting to extract text from PDF: {filepath}")
        
        try:
            # Try using pdftotext command-line tool (faster)
            self.log(1, "DEBUG: Trying pdftotext command...")
            result = subprocess.run(
                ['pdftotext', filepath, '-'],
                capture_output=True,
                text=True,
                timeout=30
            )
            if result.returncode == 0:
                text_length = len(result.stdout)
                self.log(1, f"DEBUG: pdftotext succeeded! Extracted {text_length} characters")
                if text_length == 0:
                    self.log(1, "DEBUG: WARNING - pdftotext returned 0 characters!")
                return result.stdout
            else:
                self.log(1, f"DEBUG: pdftotext failed with return code {result.returncode}")
                self.log(1, f"DEBUG: stderr: {result.stderr}")
        except FileNotFoundError:
            self.log(1, "DEBUG: pdftotext not found on PATH (is poppler-utils installed?).")
        except Exception as e:
            self.log(1, f"DEBUG: pdftotext exception: {e}")
            pass
        
        # Fallback to Python PDF library
        try:
            self.log(1, "DEBUG: Trying PyPDF2 fallback...")
            import PyPDF2
            text_content = []
            with open(filepath, 'rb') as f:
                pdf_reader = PyPDF2.PdfReader(f)
                page_count = len(pdf_reader.pages)
                self.log(1, f"DEBUG: PDF has {page_count} pages")
                for i, page in enumerate(pdf_reader.pages):
                    page_text = page.extract_text() or ""
                    text_content.append(page_text)
                    self.log(1, f"DEBUG: Page {i+1}: extracted {len(page_text)} characters")
            total_text = '\n'.join(text_content)
            self.log(1, f"DEBUG: PyPDF2 total: {len(total_text)} characters")
            return total_text
        except Exception as e:
            self.log(1, f"DEBUG: PyPDF2 exception: {e}")
            return None
    
    def load_config(self):
        """Load configuration from file"""
        if self.config_file.exists():
            self.config.read(self.config_file)
        else:
            # Create default config
            self.config['Recent'] = {}
            self.config['Recent']['paths'] = ''
            self.config['Recent']['content_patterns'] = ''
            self.config['Favorites'] = {}
            self.config['Window'] = {}
            self.config['Columns'] = {}
    
    def save_config(self):
        """Save configuration to file"""
        # Create config directory if it doesn't exist
        self.config_dir.mkdir(parents=True, exist_ok=True)
        
        with open(self.config_file, 'w') as f:
            self.config.write(f)
    
    def load_window_state(self):
        """Load window position and size"""
        if self.config.has_section('Window'):
            try:
                x = self.config.getint('Window', 'x', fallback=-1)
                y = self.config.getint('Window', 'y', fallback=-1)
                width = self.config.getint('Window', 'width', fallback=900)
                height = self.config.getint('Window', 'height', fallback=600)
                
                if x >= 0 and y >= 0:
                    self.move(x, y)
                self.resize(width, height)
            except:
                pass
    
    def save_window_state(self):
        """Save window position and size"""
        if not self.config.has_section('Window'):
            self.config.add_section('Window')
        
        x, y = self.get_position()
        width, height = self.get_size()
        
        self.config['Window']['x'] = str(x)
        self.config['Window']['y'] = str(y)
        self.config['Window']['width'] = str(width)
        self.config['Window']['height'] = str(height)
    
    def load_column_widths(self):
        """Load column widths from config"""
        # This will be called after creating the tree views
        if self.config.has_section('Columns'):
            try:
                # Result Details columns
                if hasattr(self, 'file_tree'):
                    for i, name in enumerate(['file', 'modified', 'size']):
                        width = self.config.getint('Columns', f'details_{name}', fallback=-1)
                        if width > 0:
                            column = self.file_tree.get_column(i)
                            column.set_sizing(Gtk.TreeViewColumnSizing.FIXED)
                            column.set_fixed_width(width)
                            column.set_resizable(True)  # Still allow resizing
                
                # Favorites columns (now includes search_term)
                if hasattr(self, 'favorites_tree'):
                    for i, name in enumerate(['file', 'search_term', 'path', 'modified', 'size']):
                        width = self.config.getint('Columns', f'favorites_{name}', fallback=-1)
                        if width > 0:
                            column = self.favorites_tree.get_column(i)
                            column.set_sizing(Gtk.TreeViewColumnSizing.FIXED)
                            column.set_fixed_width(width)
                            column.set_resizable(True)  # Still allow resizing
            except:
                pass
    
    def save_column_widths(self):
        """Save column widths to config"""
        if not self.config.has_section('Columns'):
            self.config.add_section('Columns')
        
        # Result Details columns
        if hasattr(self, 'file_tree'):
            for i, name in enumerate(['file', 'modified', 'size']):
                width = self.file_tree.get_column(i).get_width()
                self.config['Columns'][f'details_{name}'] = str(width)
        
        # Favorites columns (now includes search_term)
        if hasattr(self, 'favorites_tree'):
            for i, name in enumerate(['file', 'search_term', 'path', 'modified', 'size']):
                width = self.favorites_tree.get_column(i).get_width()
                self.config['Columns'][f'favorites_{name}'] = str(width)
    
    def get_recent_paths(self):
        """Get list of recent search paths"""
        paths_str = self.config.get('Recent', 'paths', fallback='')
        if paths_str:
            return [p.strip() for p in paths_str.split('|') if p.strip()]
        return []
    
    def get_recent_content_patterns(self):
        """Get list of recent content patterns"""
        patterns_str = self.config.get('Recent', 'content_patterns', fallback='')
        if patterns_str:
            return [p.strip() for p in patterns_str.split('|') if p.strip()]
        return []
    
    def get_custom_programs(self):
        """Get list of custom programs"""
        if not self.config.has_section('CustomPrograms'):
            return []
        
        programs = []
        for name in self.config['CustomPrograms']:
            cmd = self.config['CustomPrograms'][name]
            programs.append((name, cmd))
        return programs
    
    def save_custom_program(self, name, cmd):
        """Save a custom program to config"""
        if not self.config.has_section('CustomPrograms'):
            self.config.add_section('CustomPrograms')
        
        self.config['CustomPrograms'][name] = cmd
        self.save_config()
    
    def get_favorite_searches(self):
        """Get list of favorite searches"""
        print("DEBUG: get_favorite_searches called")
        if not self.config.has_section('FavoriteSearches'):
            print("DEBUG: No [FavoriteSearches] section found")
            return []
        
        print(f"DEBUG: Found [FavoriteSearches] section")
        favorites = []
        for key in self.config['FavoriteSearches']:
            print(f"DEBUG: Processing favorite search: {key}")
            # Format: name = file_pattern|content_pattern|directory
            value = self.config['FavoriteSearches'][key]
            print(f"DEBUG: Value: {value}")
            parts = value.split('|')
            print(f"DEBUG: Parts: {parts} (count: {len(parts)})")
            if len(parts) == 3:
                favorites.append({
                    'name': key,
                    'file_pattern': parts[0],
                    'content_pattern': parts[1],
                    'directory': parts[2]
                })
                print(f"DEBUG: Added favorite: {key}")
            else:
                print(f"DEBUG: Skipped (invalid format): {key}")
        
        print(f"DEBUG: Returning {len(favorites)} favorite searches")
        return favorites
    
    def save_favorite_search(self, name, file_pattern, content_pattern, directory):
        """Save a favorite search"""
        if not self.config.has_section('FavoriteSearches'):
            self.config.add_section('FavoriteSearches')
        
        # Format: name = file_pattern|content_pattern|directory
        self.config['FavoriteSearches'][name] = f"{file_pattern}|{content_pattern}|{directory}"
        self.save_config()
    
    def delete_favorite_search(self, name):
        """Delete a favorite search"""
        if self.config.has_section('FavoriteSearches') and name in self.config['FavoriteSearches']:
            del self.config['FavoriteSearches'][name]
            self.save_config()
    
    def save_window_geometry(self):
        """Save window position and size"""
        if not self.config.has_section('Window'):
            self.config.add_section('Window')
        
        # Get window position and size
        width, height = self.get_size()
        x, y = self.get_position()
        
        self.config['Window']['width'] = str(width)
        self.config['Window']['height'] = str(height)
        self.config['Window']['x'] = str(x)
        self.config['Window']['y'] = str(y)
        
        self.save_config()
    
    def add_recent_path(self, path):
        """Add a path to recent paths (keep last 5)"""
        recent = self.get_recent_paths()
        
        # Remove if already exists (to move to front)
        if path in recent:
            recent.remove(path)
        
        # Add to front
        recent.insert(0, path)
        
        # Keep only last 5
        recent = recent[:5]
        
        # Save back to config
        self.config['Recent']['paths'] = '|'.join(recent)
        self.save_config()
        
        # Update the combo box
        self.update_dir_combo(recent)
    
    def add_recent_content_pattern(self, pattern):
        """Add a content pattern to recent patterns (keep last 10)"""
        if not pattern:  # Don't save empty patterns
            return
            
        recent = self.get_recent_content_patterns()
        
        # Remove if already exists (to move to front)
        if pattern in recent:
            recent.remove(pattern)
        
        # Add to front
        recent.insert(0, pattern)
        
        # Keep only last 10
        recent = recent[:10]
        
        # Save back to config
        self.config['Recent']['content_patterns'] = '|'.join(recent)
        self.save_config()
        
        # Update the combo box
        self.update_content_combo(recent)
    
    def update_dir_combo(self, recent_paths):
        """Update the directory combo box with recent paths"""
        # Clear existing items
        self.dir_combo.remove_all()
        
        # Add recent paths
        for path in recent_paths:
            self.dir_combo.append_text(path)
        
        # Set the first one as active if available
        if recent_paths:
            self.dir_combo.set_active(0)
    
    def update_content_combo(self, recent_patterns):
        """Update the content combo box with recent patterns"""
        # Clear existing items
        self.content_combo.remove_all()
        
        # Add recent patterns
        for pattern in recent_patterns:
            self.content_combo.append_text(pattern)
        
        # Don't set active by default - leave empty for new searches
    
    def load_favorites(self):
        """Load favorites from config file with category support"""
        if not self.config.has_section('Favorites'):
            print("DEBUG: No [Favorites] section in config")
            # Create default "Uncategorized" category
            self.favorites_store.append(None, [True, "Uncategorized", "", "", "", 0, "", ""])
            return
        
        print(f"DEBUG: Loading favorites from config...")
        
        # Load category order if it exists
        category_order = []
        if self.config.has_section('FavoriteCategories'):
            order_str = self.config.get('FavoriteCategories', 'category_order', fallback='')
            if order_str:
                category_order = [c.strip() for c in order_str.split('|') if c.strip()]
        
        # If no categories defined, use "Uncategorized"
        if not category_order:
            category_order = ["Uncategorized"]
        
        # Create category nodes
        category_iters = {}
        for category in category_order:
            # Add category node: is_category=True, name=category, rest empty
            iter = self.favorites_store.append(None, [True, category, "", "", "", 0, "", ""])
            category_iters[category] = iter
        
        # Load favorites
        for key in self.config['Favorites']:
            value = self.config['Favorites'][key]
            print(f"DEBUG: Processing favorite: {key}")
            print(f"DEBUG: Value: {value}")
            
            parts = value.split('|')
            filepath = key
            search_term = ""
            category = "Uncategorized"
            
            # Handle formats: old (4), with search term (5), with category (6)
            if len(parts) == 6:
                # New format with category
                filename, search_term, modified, size_bytes, size_human, category = parts
            elif len(parts) == 5:
                # Format with search term but no category
                filename, search_term, modified, size_bytes, size_human = parts
                category = "Uncategorized"
            elif len(parts) == 4:
                # Old format
                filename, modified, size_bytes, size_human = parts
                search_term = ""
                category = "Uncategorized"
            else:
                print(f"DEBUG: Invalid parts count: {len(parts)} (expected 4, 5, or 6)")
                continue
            
            # Ensure category exists
            if category not in category_iters:
                # Category doesn't exist, add it
                iter = self.favorites_store.append(None, [True, category, "", "", "", 0, "", ""])
                category_iters[category] = iter
            
            try:
                # Try to verify file exists and update info
                if os.path.exists(filepath):
                    print(f"DEBUG: File exists: {filepath}")
                    file_size = os.path.getsize(filepath)
                    size_human = self.format_file_size(file_size)
                    modified_date = self.get_file_modified_date(filepath)
                    
                    # Add as child of category: is_category=False, name=filename, filepath, search_term, modified, size_bytes, size_human, category
                    parent_iter = category_iters[category]
                    self.favorites_store.append(parent_iter, [False, filename, filepath, search_term, modified_date, file_size, size_human, category])
                    print(f"DEBUG: Added to category '{category}': {filename}")
                else:
                    print(f"DEBUG: File NOT found: {filepath}")
                    # File doesn't exist, but keep it with old info
                    parent_iter = category_iters[category]
                    self.favorites_store.append(parent_iter, [False, filename, filepath, search_term, modified, int(size_bytes), size_human, category])
                    print(f"DEBUG: Added to category '{category}' (old info): {filename}")
            except Exception as e:
                print(f"DEBUG: Error processing favorite: {e}")
                # Try to add with whatever info we have
                try:
                    parent_iter = category_iters[category]
                    self.favorites_store.append(parent_iter, [False, filename, filepath, search_term, modified, int(size_bytes), size_human, category])
                    print(f"DEBUG: Added to category '{category}' (fallback): {filename}")
                except Exception as e2:
                    print(f"DEBUG: Failed fallback add: {e2}")
        
        # Expand all categories by default
        self.favorites_tree.expand_all()
        
        print(f"DEBUG: Favorites loading complete.")
    
    def save_favorites(self):
        """Save favorites to config file with categories"""
        # Clear existing sections
        if self.config.has_section('Favorites'):
            self.config.remove_section('Favorites')
        if self.config.has_section('FavoriteCategories'):
            self.config.remove_section('FavoriteCategories')
        
        self.config.add_section('Favorites')
        self.config.add_section('FavoriteCategories')
        
        # Collect category order
        categories = []
        
        # Iterate through tree structure
        iter = self.favorites_store.get_iter_first()
        while iter is not None:
            is_category = self.favorites_store[iter][0]
            
            if is_category:
                # This is a category node
                category_name = self.favorites_store[iter][1]
                categories.append(category_name)
                
                # Iterate through children (files in this category)
                child_iter = self.favorites_store.iter_children(iter)
                while child_iter is not None:
                    filename = self.favorites_store[child_iter][1]
                    filepath = self.favorites_store[child_iter][2]
                    search_term = self.favorites_store[child_iter][3]
                    modified = self.favorites_store[child_iter][4]
                    size_bytes = self.favorites_store[child_iter][5]
                    size_human = self.favorites_store[child_iter][6]
                    category = self.favorites_store[child_iter][7]
                    
                    # Format: filepath = filename|search_term|modified|size_bytes|size_human|category
                    self.config['Favorites'][filepath] = f"{filename}|{search_term}|{modified}|{size_bytes}|{size_human}|{category}"
                    
                    child_iter = self.favorites_store.iter_next(child_iter)
            
            iter = self.favorites_store.iter_next(iter)
        
        # Save category order
        self.config['FavoriteCategories']['category_order'] = '|'.join(categories)
        
        self.save_config()
    
    def on_add_to_favorites(self, menu_item, filepath):
        """Add a file to favorites"""
        # Check if already in favorites (search all categories)
        def check_exists(model, path, iter):
            if not model[iter][0]:  # is_category==False (it's a file)
                if model[iter][2] == filepath:  # filepath column
                    self.already_exists = True
                    return True
            return False
        
        self.already_exists = False
        self.favorites_store.foreach(check_exists)
        
        if self.already_exists:
            self.statusbar.push(self.context_id, f"File already in favorites")
            return
        
        # Check if we have room (max 100 files across all categories)
        file_count = 0
        def count_files(model, path, iter):
            if not model[iter][0]:  # is_category==False
                self.file_count += 1
            return False
        
        self.file_count = 0
        self.favorites_store.foreach(count_files)
        
        if self.file_count >= 100:
            error_dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.WARNING,
                buttons=Gtk.ButtonsType.OK,
                text="Favorites Limit Reached"
            )
            error_dialog.format_secondary_text("You can have a maximum of 100 favorites. Please remove some before adding more.")
            error_dialog.run()
            error_dialog.destroy()
            return
        
        # Add to favorites in "Uncategorized" category
        try:
            filename = os.path.basename(filepath)
            file_size = os.path.getsize(filepath)
            size_human = self.format_file_size(file_size)
            modified_date = self.get_file_modified_date(filepath)
            
            # Capture current search term if content search is enabled
            search_term = ""
            if self.content_enabled.get_active():
                search_term = self.content_combo.get_child().get_text()
            
            # Find or create "Uncategorized" category
            uncategorized_iter = None
            iter = self.favorites_store.get_iter_first()
            while iter is not None:
                if self.favorites_store[iter][0] and self.favorites_store[iter][1] == "Uncategorized":
                    uncategorized_iter = iter
                    break
                iter = self.favorites_store.iter_next(iter)
            
            if uncategorized_iter is None:
                # Create Uncategorized category
                uncategorized_iter = self.favorites_store.append(None, [True, "Uncategorized", "", "", "", 0, "", ""])
            
            # Add file: is_category=False, name=filename, filepath, search_term, modified, size_bytes, size_human, category="Uncategorized"
            self.favorites_store.append(uncategorized_iter, [False, filename, filepath, search_term, modified_date, file_size, size_human, "Uncategorized"])
            
            # Expand the category
            path = self.favorites_store.get_path(uncategorized_iter)
            self.favorites_tree.expand_row(path, False)
            
            self.save_favorites()
            self.statusbar.push(self.context_id, f"Added to favorites: {filename}")
        except Exception as e:
            error_dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="Failed to add to favorites"
            )
            error_dialog.format_secondary_text(str(e))
            error_dialog.run()
            error_dialog.destroy()
    
    @staticmethod
    def format_file_size(size_bytes):
        """Format file size in human-readable format"""
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size_bytes < 1024.0:
                return f"{size_bytes:.1f} {unit}"
            size_bytes /= 1024.0
        return f"{size_bytes:.1f} PB"
    
    @staticmethod
    def get_file_modified_date(filepath):
        """Get file modified date in YYYY-MM-DD HH:MM:SS format"""
        try:
            mtime = os.path.getmtime(filepath)
            return datetime.fromtimestamp(mtime).strftime('%Y-%m-%d %H:%M:%S')
        except:
            return "Unknown"
    
    def setup_ui(self):
        """Setup the user interface"""
        # Main container
        main_vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.add(main_vbox)
        
        # Toolbar
        toolbar = self.create_toolbar()
        main_vbox.pack_start(toolbar, False, False, 0)
        
        # Notebook for tabs
        self.notebook = Gtk.Notebook()
        main_vbox.pack_start(self.notebook, True, True, 0)
        
        # Basic search tab
        basic_tab = self.create_basic_tab()
        self.notebook.append_page(basic_tab, Gtk.Label(label="Basic"))
        
        # Advanced options tab
        advanced_tab = self.create_advanced_tab()
        self.notebook.append_page(advanced_tab, Gtk.Label(label="Advanced"))
        
        # Result Details tab (moved before Results Overview)
        results_detail_tab = self.create_results_detail_tab()
        self.notebook.append_page(results_detail_tab, Gtk.Label(label="Result Details"))
        
        # Result Overview tab
        results_overview_tab = self.create_results_overview_tab()
        self.notebook.append_page(results_overview_tab, Gtk.Label(label="Result Overview"))
        
        # Favorite Results tab
        favorites_tab = self.create_favorites_tab()
        self.notebook.append_page(favorites_tab, Gtk.Label(label="Favorite Results"))
        
        # Status bar
        self.statusbar = Gtk.Statusbar()
        self.context_id = self.statusbar.get_context_id("search")
        main_vbox.pack_start(self.statusbar, False, False, 0)
    
    def create_toolbar(self):
        """Create the toolbar"""
        toolbar = Gtk.Toolbar()
        toolbar.set_style(Gtk.ToolbarStyle.BOTH)
        
        # Add SearchBoar branding/logo with icon
        logo_item = Gtk.ToolItem()
        logo_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        logo_box.set_margin_end(15)
        
        # Try to load and add icon image
        try:
            # Try multiple possible locations for the icon
            icon_paths = [
                os.path.join(os.path.dirname(os.path.abspath(__file__)), 'searchboar-icon.png'),
                os.path.join(os.path.dirname(os.path.abspath(__file__)), 'searchboar3.png'),
                os.path.expanduser('~/.local/share/searchboar/searchboar-icon.png'),
                '/.local/share/bin/searchboar-icon.png',
                '/usr/share/pixmaps/searchboar-icon.png',
                '/usr/share/pixmaps/searchboar-icon.png'
            ]
            
            icon_loaded = False
            for icon_path in icon_paths:
                if os.path.exists(icon_path):
                    pixbuf = GdkPixbuf.Pixbuf.new_from_file(icon_path)
                    # Get the natural height of icons in the toolbar (around 48px for BOTH style)
                    # Scale to 40px to fit nicely without expanding the toolbar
                    target_height = 40
                    aspect_ratio = pixbuf.get_width() / pixbuf.get_height()
                    target_width = int(target_height * aspect_ratio)
                    scaled_pixbuf = pixbuf.scale_simple(target_width, target_height, GdkPixbuf.InterpType.BILINEAR)
                    icon_image = Gtk.Image.new_from_pixbuf(scaled_pixbuf)
                    logo_box.pack_start(icon_image, False, False, 0)
                    icon_loaded = True
                    break
            
            if not icon_loaded:
                print("SearchBoar icon not found. Place 'searchboar-icon.png' or 'searchboar3.png' in the same directory as the script.")
        except Exception as e:
            print(f"Could not load SearchBoar icon: {e}")
        
        # Create label with styled text
        logo_label = Gtk.Label()
        logo_label.set_markup('<span size="x-large" weight="bold" foreground="#5DADE2">Search</span><span size="x-large" weight="bold" foreground="#E67E22">Boar</span>')
        logo_box.pack_start(logo_label, False, False, 0)
        
        logo_item.add(logo_box)
        toolbar.insert(logo_item, -1)
        
        toolbar.insert(Gtk.SeparatorToolItem(), -1)
        
        # Home button - go to Basic tab
        home_button = Gtk.ToolButton(label="Home")
        home_button.set_icon_name("go-home")
        home_button.set_tooltip_text("Go to Basic Tab")
        home_button.connect("clicked", self.on_home)
        toolbar.insert(home_button, -1)
        
        # Search button
        self.search_button = Gtk.ToolButton(label="Search")
        self.search_button.set_icon_name("edit-find")
        self.search_button.set_tooltip_text("Start Search")
        self.search_button.connect("clicked", self.on_start_search)
        toolbar.insert(self.search_button, -1)
        
        # Stored Searches button
        fav_searches_button = Gtk.ToolButton(label="Stored Searches")
        fav_searches_button.set_icon_name("emblem-documents")
        fav_searches_button.set_tooltip_text("Manage Stored Searches")
        fav_searches_button.connect("clicked", self.on_favorite_searches)
        toolbar.insert(fav_searches_button, -1)
        
        # Favorite Results button
        fav_results_button = Gtk.ToolButton(label="Favorite Results")
        fav_results_button.set_icon_name("starred")
        fav_results_button.set_tooltip_text("View Favorite Results")
        fav_results_button.connect("clicked", self.on_show_favorite_results)
        toolbar.insert(fav_results_button, -1)
        
        toolbar.insert(Gtk.SeparatorToolItem(), -1)
        
        # Stop search
        self.stop_button = Gtk.ToolButton(stock_id=Gtk.STOCK_MEDIA_STOP)
        self.stop_button.set_tooltip_text("Stop Search")
        self.stop_button.set_sensitive(False)
        self.stop_button.connect("clicked", self.on_stop_search)
        toolbar.insert(self.stop_button, -1)
        
        toolbar.insert(Gtk.SeparatorToolItem(), -1)
        
        # Options button
        options_button = Gtk.ToolButton(label="Options")
        options_button.set_icon_name("document-properties")  # Monochrome icon
        options_button.set_tooltip_text("Click here to edit SearchBoar config file in your local text editor. Save any desired changes and restart the program to implement.")
        options_button.connect("clicked", self.on_config)
        toolbar.insert(options_button, -1)
        
        toolbar.insert(Gtk.SeparatorToolItem(), -1)
        
        # Help button
        help_button = Gtk.ToolButton(stock_id=Gtk.STOCK_HELP)
        help_button.set_tooltip_text("About SearchBoar")
        help_button.connect("clicked", self.on_help)
        toolbar.insert(help_button, -1)
        
        return toolbar
    
    def create_basic_tab(self):
        """Create the basic search tab"""
        vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        vbox.set_margin_start(10)
        vbox.set_margin_end(10)
        vbox.set_margin_top(10)
        vbox.set_margin_bottom(10)
        
        # File type checkboxes
        filetype_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        filetype_label = Gtk.Label(label="File Types:")
        filetype_label.set_size_request(100, -1)
        filetype_label.set_xalign(0)
        
        # Create checkboxes
        self.filetype_checkboxes = {}
        filetypes = ['ALL', 'MD', 'ORG', 'TXT', 'HTML', 'PDF', 'DOCX']
        
        filetype_box.pack_start(filetype_label, False, False, 0)
        
        for ft in filetypes:
            cb = Gtk.CheckButton(label=ft)
            if ft == 'ALL':
                cb.set_active(True)  # ALL is checked by default
            cb.connect("toggled", self.on_filetype_toggled)
            self.filetype_checkboxes[ft] = cb
            filetype_box.pack_start(cb, False, False, 5)
        
        vbox.pack_start(filetype_box, False, False, 0)
        
        # File name pattern
        file_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        file_label = Gtk.Label(label="Files:")
        file_label.set_size_request(100, -1)
        file_label.set_xalign(0)
        
        self.file_entry = Gtk.Entry()
        self.file_entry.set_placeholder_text(r"e.g., *.py or .*(\.c|\.h)$")
        self.file_entry.set_text(".*")
        
        file_wizard_btn = Gtk.Button(label="Expr. Wizard")
        file_wizard_btn.connect("clicked", self.on_file_wizard)
        
        file_box.pack_start(file_label, False, False, 0)
        file_box.pack_start(self.file_entry, True, True, 0)
        file_box.pack_start(file_wizard_btn, False, False, 0)
        vbox.pack_start(file_box, False, False, 0)
        
        # Content pattern
        content_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        
        self.content_enabled = Gtk.CheckButton()
        self.content_enabled.set_active(True)
        
        content_label = Gtk.Label(label="Containing:")
        content_label.set_size_request(85, -1)
        content_label.set_xalign(0)
        
        # Create editable combo box for recent content patterns
        self.content_combo = Gtk.ComboBoxText.new_with_entry()
        self.content_combo.get_child().set_placeholder_text("Text or regex to search for in files")
        
        # Connect Enter key to start search
        self.content_combo.get_child().connect("activate", self.on_search_field_activate)
        
        # Populate with recent patterns
        recent_patterns = self.get_recent_content_patterns()
        for pattern in recent_patterns:
            self.content_combo.append_text(pattern)
        
        content_wizard_btn = Gtk.Button(label="Expr. Wizard")
        content_wizard_btn.connect("clicked", self.on_content_wizard)
        
        content_box.pack_start(self.content_enabled, False, False, 0)
        content_box.pack_start(content_label, False, False, 0)
        content_box.pack_start(self.content_combo, True, True, 0)
        content_box.pack_start(content_wizard_btn, False, False, 0)
        vbox.pack_start(content_box, False, False, 0)
        
        # Look in directory
        dir_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        dir_label = Gtk.Label(label="Look in:")
        dir_label.set_size_request(100, -1)
        dir_label.set_xalign(0)
        
        # Create editable combo box for recent paths
        self.dir_combo = Gtk.ComboBoxText.new_with_entry()
        
        # Connect Enter key to start search
        self.dir_combo.get_child().connect("activate", self.on_search_field_activate)
        
        # Populate with recent paths
        recent_paths = self.get_recent_paths()
        for path in recent_paths:
            self.dir_combo.append_text(path)
        
        # Set current directory or home as default
        if recent_paths:
            self.dir_combo.set_active(0)
        else:
            self.dir_combo.get_child().set_text(str(Path.home()))
        
        dir_button = Gtk.Button(label="Browse...")
        dir_button.connect("clicked", self.on_browse_directory)
        
        dir_box.pack_start(dir_label, False, False, 0)
        dir_box.pack_start(self.dir_combo, True, True, 0)
        dir_box.pack_start(dir_button, False, False, 0)
        vbox.pack_start(dir_box, False, False, 0)
        
        # Options
        options_frame = Gtk.Frame(label="Options")
        options_frame.set_margin_top(10)
        
        options_vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        options_vbox.set_margin_start(10)
        options_vbox.set_margin_end(10)
        options_vbox.set_margin_top(10)
        options_vbox.set_margin_bottom(10)
        
        self.recursive_check = Gtk.CheckButton(label="Search in subdirectories")
        self.recursive_check.set_active(True)
        options_vbox.pack_start(self.recursive_check, False, False, 0)
        
        self.case_check = Gtk.CheckButton(label="Case sensitive")
        options_vbox.pack_start(self.case_check, False, False, 0)
        
        self.hidden_check = Gtk.CheckButton(label="Include hidden files")
        options_vbox.pack_start(self.hidden_check, False, False, 0)
        
        options_frame.add(options_vbox)
        vbox.pack_start(options_frame, False, False, 0)        
        # =========================
        # Search Help (collapsible)
        # =========================
        help_expander = Gtk.Expander(label="Search Help")
        help_expander.set_expanded(False)

        # Tighter spacing for a compact footprint
        help_grid = Gtk.Grid(column_spacing=18, row_spacing=0,
                             margin_start=6, margin_end=6,
                             margin_top=4, margin_bottom=4)

        help_left = (
            "Common Searches\n"
            "• epicurus → finds this word anywhere\n"
            "• fat and sleek → exact phrase (words together)\n"
            "• Case sensitive → matches capitalization exactly\n"
        )

        help_right = (
            "Regex Recipes (optional)\n"
            "• fear|death → either word\n"
            "• (?s)(?=.*fat)(?=.*sleek) → both words anywhere in the file\n"
            "• fat.*sleek → words in this order (same line)\n"
            "• \\bfear\\b → whole word only\n"
            "• fear → partial match (inside other words too)\n"
            "• ^Epicurus → line begins with\n"
            "• pain$ → line ends with\n"
        )


        left_label = Gtk.Label(label=help_left)
        left_label.set_xalign(0)
        left_label.set_yalign(0)
        left_label.set_selectable(False)
        left_label.set_margin_top(0)
        left_label.set_margin_bottom(0)

        right_label = Gtk.Label(label=help_right)
        right_label.set_xalign(0)
        right_label.set_yalign(0)
        right_label.set_selectable(False)
        right_label.set_margin_top(0)
        right_label.set_margin_bottom(0)

        help_grid.attach(left_label, 0, 0, 1, 1)
        help_grid.attach(right_label, 1, 0, 1, 1)

        help_expander.add(help_grid)

        # Center the expander within the tab
        help_center_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL)
        help_center_box.set_halign(Gtk.Align.CENTER)
        help_center_box.pack_start(help_expander, False, False, 0)

        vbox.pack_start(help_center_box, False, False, 0)

        return vbox
    
    def create_advanced_tab(self):
        """Create the advanced options tab"""
        vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        vbox.set_margin_start(10)
        vbox.set_margin_end(10)
        vbox.set_margin_top(10)
        vbox.set_margin_bottom(10)
        
        # Context lines
        context_frame = Gtk.Frame(label="Context Lines")
        context_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        context_box.set_margin_start(10)
        context_box.set_margin_end(10)
        context_box.set_margin_top(10)
        context_box.set_margin_bottom(10)
        
        before_label = Gtk.Label(label="Lines before:")
        self.before_spin = Gtk.SpinButton()
        self.before_spin.set_range(0, 10)
        self.before_spin.set_increments(1, 1)
        self.before_spin.set_value(2)
        
        after_label = Gtk.Label(label="Lines after:")
        self.after_spin = Gtk.SpinButton()
        self.after_spin.set_range(0, 10)
        self.after_spin.set_increments(1, 1)
        self.after_spin.set_value(2)
        
        context_box.pack_start(before_label, False, False, 0)
        context_box.pack_start(self.before_spin, False, False, 0)
        context_box.pack_start(after_label, False, False, 0)
        context_box.pack_start(self.after_spin, False, False, 0)
        
        context_frame.add(context_box)
        vbox.pack_start(context_frame, False, False, 0)
        
        # File size filter
        size_frame = Gtk.Frame(label="File Size Filter")
        size_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        size_box.set_margin_start(10)
        size_box.set_margin_end(10)
        size_box.set_margin_top(10)
        size_box.set_margin_bottom(10)
        
        min_size_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        min_size_label = Gtk.Label(label="Min size (KB):")
        self.min_size_spin = Gtk.SpinButton()
        self.min_size_spin.set_range(0, 1000000)
        self.min_size_spin.set_increments(1, 100)
        self.min_size_spin.set_value(0)
        min_size_box.pack_start(min_size_label, False, False, 0)
        min_size_box.pack_start(self.min_size_spin, False, False, 0)
        
        max_size_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        max_size_label = Gtk.Label(label="Max size (KB):")
        self.max_size_spin = Gtk.SpinButton()
        self.max_size_spin.set_range(0, 1000000)
        self.max_size_spin.set_increments(1, 100)
        self.max_size_spin.set_value(0)
        max_size_box.pack_start(max_size_label, False, False, 0)
        max_size_box.pack_start(self.max_size_spin, False, False, 0)
        
        size_box.pack_start(min_size_box, False, False, 0)
        size_box.pack_start(max_size_box, False, False, 0)
        
        size_frame.add(size_box)
        vbox.pack_start(size_frame, False, False, 0)
        
        # Exclude patterns
        exclude_frame = Gtk.Frame(label="Exclude Patterns")
        exclude_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        exclude_box.set_margin_start(10)
        exclude_box.set_margin_end(10)
        exclude_box.set_margin_top(10)
        exclude_box.set_margin_bottom(10)
        
        exclude_label = Gtk.Label(label="Exclude files matching (comma-separated):")
        exclude_label.set_xalign(0)
        self.exclude_entry = Gtk.Entry()
        self.exclude_entry.set_placeholder_text("e.g., *.pyc,*.o,*.tmp")
        
        exclude_box.pack_start(exclude_label, False, False, 0)
        exclude_box.pack_start(self.exclude_entry, False, False, 0)
        
        exclude_frame.add(exclude_box)
        vbox.pack_start(exclude_frame, False, False, 0)
        
        return vbox
    
    def create_results_overview_tab(self):
        """Create the results overview tab with clickable list entries"""
        vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        
        overview_label = Gtk.Label(label="<b>Result Overview - First Match Per File</b>")
        overview_label.set_use_markup(True)
        overview_label.set_xalign(0)
        overview_label.set_margin_start(6)
        overview_label.set_margin_top(6)
        vbox.pack_start(overview_label, False, False, 0)
        
        # Create ListBox with single-click to select, double-click to open
        self.overview_listbox = Gtk.ListBox()
        self.overview_listbox.set_selection_mode(Gtk.SelectionMode.SINGLE)
        self.overview_listbox.set_activate_on_single_click(False)  # CRITICAL: Only double-click activates!
        
        # Connect signals
        self.overview_listbox.connect("row-activated", self.on_overview_row_activated)  # Double-click
        self.overview_listbox.connect("button-press-event", self.on_overview_listbox_button_press)  # Right-click
        self.overview_listbox.connect("row-selected", self.on_overview_row_selected)  # Selection changed
        
        overview_scroll = Gtk.ScrolledWindow()
        overview_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        overview_scroll.add(self.overview_listbox)
        
        vbox.pack_start(overview_scroll, True, True, 0)
        
        return vbox
    
    def create_results_detail_tab(self):
        """Create the result details tab"""
        paned = Gtk.Paned(orientation=Gtk.Orientation.HORIZONTAL)
        
        # Left side: File list
        left_vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        
        file_label = Gtk.Label(label="<b>Files Found</b>")
        file_label.set_use_markup(True)
        file_label.set_xalign(0)
        left_vbox.pack_start(file_label, False, False, 0)
        
        # File list store: filename, full path, modified date, size (bytes), size (human readable)
        self.file_store = Gtk.ListStore(str, str, str, int, str)
        
        # Create sorted model
        self.file_sorted = Gtk.TreeModelSort(model=self.file_store)
        self.file_tree = Gtk.TreeView(model=self.file_sorted)
        self.file_tree.set_headers_visible(True)
        self.file_tree.set_headers_clickable(True)
        
        # Filename column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("File", renderer, text=0)
        column.set_sort_column_id(0)
        column.set_resizable(True)
        column.set_visible(True)
        column.set_min_width(150)
        self.file_tree.append_column(column)
        
        # Modified date column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Modified", renderer, text=2)
        column.set_sort_column_id(2)
        column.set_resizable(True)
        column.set_visible(True)
        column.set_min_width(150)
        self.file_tree.append_column(column)
        
        # Size column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Size", renderer, text=4)
        column.set_sort_column_id(3)  # Sort by bytes (column 3), display human readable (column 4)
        column.set_resizable(True)
        column.set_visible(True)
        column.set_min_width(100)
        self.file_tree.append_column(column)
        
        self.file_tree.connect("cursor-changed", self.on_file_selected)
        self.file_tree.connect("button-press-event", self.on_file_right_click)
        self.file_tree.connect("key-press-event", self.on_file_tree_key_press)
        self.file_tree.connect("row-activated", self.on_file_tree_double_click)
        
        file_scroll = Gtk.ScrolledWindow()
        file_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        file_scroll.add(self.file_tree)
        
        left_vbox.pack_start(file_scroll, True, True, 0)
        paned.add1(left_vbox)
        
        # Right side: Content matches
        right_vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        
        content_label = Gtk.Label(label="<b>Content Matches</b>")
        content_label.set_use_markup(True)
        content_label.set_xalign(0)
        right_vbox.pack_start(content_label, False, False, 0)
        
        self.content_view = Gtk.TextView()
        self.content_view.set_editable(False)
        self.content_view.set_wrap_mode(Gtk.WrapMode.WORD)  # Enable word wrapping!
        self.content_buffer = self.content_view.get_buffer()
        
        # Create tags for highlighting
        self.match_tag = self.content_buffer.create_tag(
            "match",
            background="#5DADE2",  # Sky blue background
            foreground="white",     # White text for readability
            weight=Pango.Weight.BOLD
        )
        
        self.line_number_tag = self.content_buffer.create_tag(
            "line_number",
            foreground="#5DADE2",  # Sky blue for line numbers
            weight=Pango.Weight.BOLD
        )
        
        content_scroll = Gtk.ScrolledWindow()
        content_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        content_scroll.add(self.content_view)
        
        right_vbox.pack_start(content_scroll, True, True, 0)
        paned.add2(right_vbox)
        
        paned.set_position(300)
        
        return paned
    
    def create_favorites_tab(self):
        """Create the favorites tab with categories"""
        vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        
        fav_label = Gtk.Label(label="<b>Favorite Files</b>")
        fav_label.set_use_markup(True)
        fav_label.set_xalign(0)
        fav_label.set_margin_start(6)
        fav_label.set_margin_top(6)
        vbox.pack_start(fav_label, False, False, 0)
        
        # Category management buttons
        button_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        button_box.set_margin_start(6)
        button_box.set_margin_end(6)
        
        add_cat_btn = Gtk.Button(label="Add Category")
        add_cat_btn.connect("clicked", self.on_add_category)
        button_box.pack_start(add_cat_btn, False, False, 0)
        
        rename_cat_btn = Gtk.Button(label="Rename Category")
        rename_cat_btn.connect("clicked", self.on_rename_category)
        button_box.pack_start(rename_cat_btn, False, False, 0)
        
        move_to_cat_btn = Gtk.Button(label="Move to Category")
        move_to_cat_btn.connect("clicked", self.on_move_to_category)
        button_box.pack_start(move_to_cat_btn, False, False, 0)
        
        delete_cat_btn = Gtk.Button(label="Delete Category")
        delete_cat_btn.connect("clicked", self.on_delete_category)
        button_box.pack_start(delete_cat_btn, False, False, 0)
        
        vbox.pack_start(button_box, False, False, 0)
        
        # Favorites tree store: is_category, name/filename, filepath, search_term, modified, size_bytes, size_human, category_name
        # For categories: is_category=True, name=category_name, rest are ""
        # For files: is_category=False, name=filename, rest are file data
        self.favorites_store = Gtk.TreeStore(bool, str, str, str, str, int, str, str)
        
        self.favorites_tree = Gtk.TreeView(model=self.favorites_store)
        self.favorites_tree.set_headers_visible(True)
        self.favorites_tree.set_headers_clickable(True)
        self.favorites_tree.set_enable_tree_lines(True)
        
        # Name column (shows category name or filename)
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("File Name", renderer, text=1)
        column.set_resizable(True)
        column.set_visible(True)
        column.set_min_width(200)
        self.favorites_tree.append_column(column)
        
        # Search Term column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Search Term", renderer, text=3)
        column.set_resizable(True)
        column.set_visible(True)
        column.set_min_width(150)
        self.favorites_tree.append_column(column)
        
        # Path column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Path", renderer, text=2)
        column.set_resizable(True)
        column.set_visible(True)
        column.set_min_width(300)
        self.favorites_tree.append_column(column)
        
        # Modified date column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Modified", renderer, text=4)
        column.set_resizable(True)
        column.set_visible(True)
        column.set_min_width(150)
        self.favorites_tree.append_column(column)
        
        # Size column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Size", renderer, text=6)
        column.set_resizable(True)
        column.set_visible(True)
        column.set_min_width(100)
        self.favorites_tree.append_column(column)
        
        # Connect events
        self.favorites_tree.connect("button-press-event", self.on_favorites_right_click)
        self.favorites_tree.connect("key-press-event", self.on_favorites_key_press)
        self.favorites_tree.connect("row-activated", self.on_favorites_double_click)
        
        # Enable drag and drop for reordering
        self.favorites_tree.set_reorderable(True)  # Enable built-in reordering!
        
        # Connect to model's row-deleted signal to save when reordered
        self.favorites_store.connect("row-deleted", self.on_favorites_reordered)
        
        fav_scroll = Gtk.ScrolledWindow()
        fav_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        fav_scroll.add(self.favorites_tree)
        
        vbox.pack_start(fav_scroll, True, True, 0)
        
        # Load favorites from config
        self.load_favorites()
        
        return vbox
    
    def on_file_wizard(self, button):
        """Show regex builder for file names"""
        dialog = RegexBuilderDialog(self, "File Name Pattern Builder")
        response = dialog.run()
        
        if response == Gtk.ResponseType.OK:
            regex = dialog.get_regex()
            self.file_entry.set_text(regex)
        
        dialog.destroy()
    
    def on_content_wizard(self, button):
        """Show regex builder for content"""
        dialog = RegexBuilderDialog(self, "Content Pattern Builder")
        response = dialog.run()
        
        if response == Gtk.ResponseType.OK:
            regex = dialog.get_regex()
            self.content_combo.get_child().set_text(regex)
        
        dialog.destroy()
    
    def on_browse_directory(self, button):
        """Browse for directory"""
        dialog = Gtk.FileChooserDialog(
            title="Select Directory",
            parent=self,
            action=Gtk.FileChooserAction.SELECT_FOLDER
        )
        dialog.add_buttons(
            Gtk.STOCK_CANCEL,
            Gtk.ResponseType.CANCEL,
            Gtk.STOCK_OPEN,
            Gtk.ResponseType.OK,
        )
        
        response = dialog.run()
        if response == Gtk.ResponseType.OK:
            selected_path = dialog.get_filename()
            self.dir_combo.get_child().set_text(selected_path)
        
        dialog.destroy()
    
    def on_new_search(self, button):
        """Reset search criteria"""
        self.file_entry.set_text(".*")
        self.content_combo.get_child().set_text("")
        recent_paths = self.get_recent_paths()
        if recent_paths:
            self.dir_combo.set_active(0)
        else:
            self.dir_combo.get_child().set_text(str(Path.home()))
        self.recursive_check.set_active(True)
        self.case_check.set_active(False)
        self.hidden_check.set_active(False)
        self.before_spin.set_value(2)
        self.after_spin.set_value(2)
        self.min_size_spin.set_value(0)
        self.max_size_spin.set_value(0)
        self.exclude_entry.set_text("")
        self.file_store.clear()
        self.content_buffer.set_text("")
        
        # Clear overview ListBox
        for child in self.overview_listbox.get_children():
            self.overview_listbox.remove(child)
        
        if hasattr(self, 'overview_file_map'):
            self.overview_file_map.clear()
        self.statusbar.push(self.context_id, "Ready")
    
    def on_save_criteria(self, button):
        """Save search criteria to file"""
        dialog = Gtk.FileChooserDialog(
            title="Save Search Criteria",
            parent=self,
            action=Gtk.FileChooserAction.SAVE
        )
        dialog.add_buttons(
            Gtk.STOCK_CANCEL,
            Gtk.ResponseType.CANCEL,
            Gtk.STOCK_SAVE,
            Gtk.ResponseType.OK,
        )
        dialog.set_do_overwrite_confirmation(True)
        dialog.set_current_name("search_criteria.json")
        
        response = dialog.run()
        if response == Gtk.ResponseType.OK:
            criteria = {
                'file_pattern': self.file_entry.get_text(),
                'content_pattern': self.content_combo.get_child().get_text(),
                'content_enabled': self.content_enabled.get_active(),
                'directory': self.dir_combo.get_child().get_text(),
                'recursive': self.recursive_check.get_active(),
                'case_sensitive': self.case_check.get_active(),
                'include_hidden': self.hidden_check.get_active(),
                'context_before': self.before_spin.get_value_as_int(),
                'context_after': self.after_spin.get_value_as_int(),
                'min_size': self.min_size_spin.get_value_as_int(),
                'max_size': self.max_size_spin.get_value_as_int(),
                'exclude_pattern': self.exclude_entry.get_text()
            }
            
            with open(dialog.get_filename(), 'w') as f:
                json.dump(criteria, f, indent=2)
            
            self.statusbar.push(self.context_id, f"Criteria saved to {dialog.get_filename()}")
        
        dialog.destroy()
    
    def on_open_criteria(self, button):
        """Load search criteria from file"""
        dialog = Gtk.FileChooserDialog(
            title="Open Search Criteria",
            parent=self,
            action=Gtk.FileChooserAction.OPEN
        )
        dialog.add_buttons(
            Gtk.STOCK_CANCEL,
            Gtk.ResponseType.CANCEL,
            Gtk.STOCK_OPEN,
            Gtk.ResponseType.OK,
        )
        
        filter_json = Gtk.FileFilter()
        filter_json.set_name("JSON files")
        filter_json.add_pattern("*.json")
        dialog.add_filter(filter_json)
        
        response = dialog.run()
        if response == Gtk.ResponseType.OK:
            try:
                with open(dialog.get_filename(), 'r') as f:
                    criteria = json.load(f)
                
                self.file_entry.set_text(criteria.get('file_pattern', '.*'))
                self.content_combo.get_child().set_text(criteria.get('content_pattern', ''))
                self.content_enabled.set_active(criteria.get('content_enabled', True))
                self.dir_combo.get_child().set_text(criteria.get('directory', str(Path.home())))
                self.recursive_check.set_active(criteria.get('recursive', True))
                self.case_check.set_active(criteria.get('case_sensitive', False))
                self.hidden_check.set_active(criteria.get('include_hidden', False))
                self.before_spin.set_value(criteria.get('context_before', 0))
                self.after_spin.set_value(criteria.get('context_after', 0))
                self.min_size_spin.set_value(criteria.get('min_size', 0))
                self.max_size_spin.set_value(criteria.get('max_size', 0))
                self.exclude_entry.set_text(criteria.get('exclude_pattern', ''))
                
                self.statusbar.push(self.context_id, f"Criteria loaded from {dialog.get_filename()}")
            except Exception as e:
                error_dialog = Gtk.MessageDialog(
                    transient_for=self,
                    flags=0,
                    message_type=Gtk.MessageType.ERROR,
                    buttons=Gtk.ButtonsType.OK,
                    text="Error loading criteria"
                )
                error_dialog.format_secondary_text(str(e))
                error_dialog.run()
                error_dialog.destroy()
        
        dialog.destroy()
    
    def on_start_search(self, button):
        """Start the search"""
        # Validate inputs
        search_dir = self.dir_combo.get_child().get_text()
        if not os.path.isdir(search_dir):
            error_dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="Invalid directory"
            )
            error_dialog.format_secondary_text(f"The directory '{search_dir}' does not exist.")
            error_dialog.run()
            error_dialog.destroy()
            return
        
        # Add to recent paths
        self.add_recent_path(search_dir)
        
        # Add to recent content patterns if content search is enabled
        if self.content_enabled.get_active():
            content_pattern = self.content_combo.get_child().get_text()
            self.add_recent_content_pattern(content_pattern)
        
        # Clear previous results
        self.file_store.clear()
        self.content_buffer.set_text("")
        
        # Clear overview ListBox
        for child in self.overview_listbox.get_children():
            self.overview_listbox.remove(child)
        
        if hasattr(self, 'overview_file_map'):
            self.overview_file_map.clear()
        
        # Switch to result details tab (index 2: Basic=0, Advanced=1, Result Details=2)
        self.notebook.set_current_page(2)
        
        # Update UI
        self.search_button.set_sensitive(False)
        self.stop_button.set_sensitive(True)
        self.statusbar.push(self.context_id, "Searching...")
        
        # Start search in thread
        self.stop_search = False
        self.search_thread = threading.Thread(target=self.perform_search)
        self.search_thread.daemon = True
        self.search_thread.start()
    
    def on_stop_search(self, button):
        """Stop the search"""
        self.stop_search = True
        self.statusbar.push(self.context_id, "Stopping search...")
    
    def on_favorite_searches(self, button):
        """Show stored searches dialog"""
        dialog = Gtk.Dialog(
            title="Stored Searches",
            transient_for=self,
            flags=0
        )
        dialog.add_buttons(
            Gtk.STOCK_CLOSE, Gtk.ResponseType.CLOSE
        )
        dialog.set_default_size(600, 400)
        
        content_area = dialog.get_content_area()
        content_area.set_spacing(10)
        content_area.set_margin_start(10)
        content_area.set_margin_end(10)
        content_area.set_margin_top(10)
        content_area.set_margin_bottom(10)
        
        # Top button box
        button_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        
        save_btn = Gtk.Button(label="Save Current Search")
        save_btn.connect("clicked", self.on_save_favorite_search, dialog)
        button_box.pack_start(save_btn, False, False, 0)
        
        load_btn = Gtk.Button(label="Load Selected")
        load_btn.connect("clicked", self.on_load_favorite_search, dialog)
        button_box.pack_start(load_btn, False, False, 0)
        
        delete_btn = Gtk.Button(label="Delete Selected")
        delete_btn.connect("clicked", self.on_delete_favorite_search_from_dialog, dialog)
        button_box.pack_start(delete_btn, False, False, 0)
        
        content_area.pack_start(button_box, False, False, 0)
        
        # List of favorite searches
        scrolled = Gtk.ScrolledWindow()
        scrolled.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        
        # Store: name, file_pattern, content_pattern, directory
        self.fav_search_store = Gtk.ListStore(str, str, str, str)
        
        # Load favorites
        for fav in self.get_favorite_searches():
            self.fav_search_store.append([
                fav['name'],
                fav['file_pattern'],
                fav['content_pattern'],
                fav['directory']
            ])
        
        self.fav_search_tree = Gtk.TreeView(model=self.fav_search_store)
        
        # Name column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Name", renderer, text=0)
        column.set_resizable(True)
        column.set_min_width(150)
        self.fav_search_tree.append_column(column)
        
        # File Pattern column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("File Pattern", renderer, text=1)
        column.set_resizable(True)
        column.set_min_width(100)
        self.fav_search_tree.append_column(column)
        
        # Content Pattern column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Content Pattern", renderer, text=2)
        column.set_resizable(True)
        column.set_min_width(150)
        self.fav_search_tree.append_column(column)
        
        # Directory column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Directory", renderer, text=3)
        column.set_resizable(True)
        column.set_min_width(150)
        self.fav_search_tree.append_column(column)
        
        scrolled.add(self.fav_search_tree)
        content_area.pack_start(scrolled, True, True, 0)
        
        dialog.show_all()
        dialog.run()
        dialog.destroy()
    
    def on_save_favorite_search(self, button, parent_dialog):
        """Save current search as favorite"""
        # Get current search criteria
        file_pattern = self.file_entry.get_text()
        content_pattern = self.content_combo.get_child().get_text()
        directory = self.dir_combo.get_child().get_text()
        
        # Ask for name
        name_dialog = Gtk.MessageDialog(
            transient_for=parent_dialog,
            flags=0,
            message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL,
            text="Save Favorite Search"
        )
        name_dialog.format_secondary_text("Enter a name for this search:")
        
        # Add entry field
        content_area = name_dialog.get_content_area()
        entry = Gtk.Entry()
        entry.set_placeholder_text("Search name")
        
        # Connect Enter key to activate default response (OK)
        entry.connect("activate", lambda e: name_dialog.response(Gtk.ResponseType.OK))
        
        content_area.pack_start(entry, False, False, 0)
        
        name_dialog.show_all()
        response = name_dialog.run()
        name = entry.get_text().strip()
        name_dialog.destroy()
        
        if response == Gtk.ResponseType.OK and name:
            # Save to config
            self.save_favorite_search(name, file_pattern, content_pattern, directory)
            
            # Add to tree view
            self.fav_search_store.append([name, file_pattern, content_pattern, directory])
            
            self.statusbar.push(self.context_id, f"Saved favorite search: {name}")
    
    def on_load_favorite_search(self, button, parent_dialog):
        """Load selected favorite search"""
        selection = self.fav_search_tree.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return
        
        # Get values
        name = model[iter][0]
        file_pattern = model[iter][1]
        content_pattern = model[iter][2]
        directory = model[iter][3]
        
        # Set in UI
        self.file_entry.set_text(file_pattern)
        self.content_combo.get_child().set_text(content_pattern)
        self.dir_combo.get_child().set_text(directory)
        
        parent_dialog.response(Gtk.ResponseType.CLOSE)
        
        self.statusbar.push(self.context_id, f"Loaded favorite search: {name}")
    
    def on_delete_favorite_search_from_dialog(self, button, parent_dialog):
        """Delete selected favorite search"""
        selection = self.fav_search_tree.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return
        
        name = model[iter][0]
        
        # Confirm deletion
        confirm_dialog = Gtk.MessageDialog(
            transient_for=parent_dialog,
            flags=0,
            message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.YES_NO,
            text="Delete Favorite Search"
        )
        confirm_dialog.format_secondary_text(f"Are you sure you want to delete '{name}'?")
        
        response = confirm_dialog.run()
        confirm_dialog.destroy()
        
        if response == Gtk.ResponseType.YES:
            # Delete from config
            self.delete_favorite_search(name)
            
            # Remove from tree view
            model.remove(iter)
            
            self.statusbar.push(self.context_id, f"Deleted favorite search: {name}")
    
    def on_show_favorite_results(self, button):
        """Switch to Favorite Results tab"""
        # Tab index 4: Basic=0, Advanced=1, Result Details=2, Results Overview=3, Favorite Results=4
        self.notebook.set_current_page(4)
    
    def on_home(self, button):
        """Go to Basic tab"""
        self.notebook.set_current_page(0)
    
    def on_filetype_toggled(self, checkbox):
        """Handle file type checkbox toggles"""
        label = checkbox.get_label()
        
        if label == 'ALL':
            if checkbox.get_active():
                # Uncheck all others
                for ft, cb in self.filetype_checkboxes.items():
                    if ft != 'ALL':
                        cb.set_active(False)
                # Set pattern to match all
                self.file_entry.set_text(".*")
        else:
            # If any specific type is checked, uncheck ALL
            if checkbox.get_active():
                self.filetype_checkboxes['ALL'].set_active(False)
            
            # Build pattern from checked boxes
            checked = []
            for ft, cb in self.filetype_checkboxes.items():
                if ft != 'ALL' and cb.get_active():
                    if ft == 'MD':
                        checked.append('\\.md')
                    elif ft == 'ORG':
                        checked.append('\\.org')
                    elif ft == 'TXT':
                        checked.append('\\.txt')
                    elif ft == 'HTML':
                        checked.append('\\.(html|htm)')
                    elif ft == 'PDF':
                        checked.append('\\.pdf')
                    elif ft == 'DOCX':
                        checked.append('\\.docx')
            
            if checked:
                # Create pattern: .*\.(ext1|ext2|ext3)$
                pattern = f".*({'|'.join(checked)})$"
                self.file_entry.set_text(pattern)
            else:
                # Nothing checked, revert to ALL
                self.filetype_checkboxes['ALL'].set_active(True)
                self.file_entry.set_text(".*")
    
    def on_config(self, button):
        """Open config file in default text editor"""
        import subprocess
        config_path = os.path.join(self.config_dir, 'config.ini')
        
        try:
            # Try to open with default text editor
            subprocess.Popen(['xdg-open', config_path])
        except Exception as e:
            # Show error dialog
            error_dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="Failed to open config file"
            )
            error_dialog.format_secondary_text(
                f"Could not open {config_path}\n\n"
                f"Error: {str(e)}\n\n"
                f"You can manually open the file at:\n{config_path}"
            )
            error_dialog.run()
            error_dialog.destroy()
    
    def on_help(self, button):
        """Show help/about dialog"""
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.INFO,
            buttons=Gtk.ButtonsType.OK,
            text=f"SearchBoar v. {VERSION}"
        )
        dialog.format_secondary_text(
            "SearchBoar was developed for the study of Epicurus as pursued at EpicureanFriends.com"
        )
        dialog.set_title("About SearchBoar")
        
        # Center the dialog
        dialog.set_position(Gtk.WindowPosition.CENTER_ON_PARENT)
        
        dialog.run()
        dialog.destroy()
    
    def on_window_close(self, window, event):
        """Handle window close event - save settings"""
        self.save_window_state()
        self.save_column_widths()
        self.save_config()
        return False  # Allow window to close
    
    def on_search_field_activate(self, entry):
        """Handle Enter key in search fields"""
        self.on_start_search(None)
    
    def perform_search(self):
        """Perform the actual search (runs in separate thread)"""
        try:
            print("\n" + "="*70)
            print("SEARCH STARTED")
            print("="*70)
            
            # Get search parameters
            file_pattern = self.file_entry.get_text()
            content_pattern = self.content_combo.get_child().get_text() if self.content_enabled.get_active() else None
            search_dir = self.dir_combo.get_child().get_text()
            recursive = self.recursive_check.get_active()
            case_sensitive = self.case_check.get_active()
            include_hidden = self.hidden_check.get_active()
            context_before = self.before_spin.get_value_as_int()
            context_after = self.after_spin.get_value_as_int()
            min_size = self.min_size_spin.get_value_as_int() * 1024  # Convert to bytes
            max_size = self.max_size_spin.get_value_as_int() * 1024
            exclude_patterns = [p.strip() for p in self.exclude_entry.get_text().split(',') if p.strip()]
            
            # Log search parameters
            print(f"Search directory: {search_dir}")
            print(f"File pattern: {file_pattern}")
            print(f"Content pattern: {content_pattern if content_pattern else '(none - filename only)'}")
            print(f"Recursive: {recursive}")
            print(f"Case sensitive: {case_sensitive}")
            print(f"Include hidden: {include_hidden}")
            print(f"Min size: {min_size} bytes")
            print(f"Max size: {max_size} bytes" if max_size > 0 else "Max size: (unlimited)")
            print(f"Exclude patterns: {exclude_patterns if exclude_patterns else '(none)'}")
            print("-"*70)
            
            # Compile regex patterns
            flags = 0 if case_sensitive else re.IGNORECASE
            try:
                file_regex = re.compile(file_pattern, flags)
                content_regex = re.compile(content_pattern, flags) if content_pattern else None
                exclude_regexes = [re.compile(p, flags) for p in exclude_patterns]
                print(f"Regex compilation successful")
            except re.error as e:
                print(f"ERROR: Regex compilation failed: {e}")
                GLib.idle_add(self.show_regex_error, str(e))
                GLib.idle_add(self.search_complete, 0, 0)
                return
            
            files_found = 0
            files_searched = 0
            
            # If using ripgrep for content search, note that ripgrep cannot search inside PDFs/DOCX.
            # If the filename pattern appears to target PDFs or DOCX, bypass ripgrep so extractor-based searching runs.
            wants_pdf = self._pattern_mentions_ext(file_pattern, '.pdf')
            wants_docx = self._pattern_mentions_ext(file_pattern, '.docx')
            wants_nontext = wants_pdf or wants_docx

            if self.has_ripgrep and content_pattern and recursive and not wants_nontext:
                self.log(1, "Using ripgrep for fast text search (non-PDF/DOCX).")
                files_found = self.search_with_ripgrep(
                    search_dir, file_regex, content_pattern, case_sensitive,
                    context_before, context_after, exclude_regexes,
                    include_hidden, min_size, max_size
                )
                # If ripgrep succeeded (returned >= 0), use those results
                # If it failed (returned -1), fall through to Python search
                if files_found >= 0:
                    # Note: ripgrep doesn't tell us total files searched, only found
                    GLib.idle_add(self.search_complete, files_found, files_found)
                    return
            elif self.has_ripgrep and content_pattern and recursive and wants_nontext:
                self.log(1, "NOTE: PDF/DOCX detected in filename pattern; bypassing ripgrep so extractor-based searching can run.")
            
            # Collect files to search
            files_to_search = []
            for root, dirs, files in os.walk(search_dir):
                if self.stop_search:
                    break
                
                # Filter hidden directories if needed
                if not include_hidden:
                    dirs[:] = [d for d in dirs if not d.startswith('.')]
                
                for filename in files:
                    if self.stop_search:
                        break
                    
                    # Skip hidden files if needed
                    if not include_hidden and filename.startswith('.'):
                        continue
                    
                    filepath = os.path.join(root, filename)
                    
                    # Check file size
                    try:
                        file_size = os.path.getsize(filepath)
                        if min_size > 0 and file_size < min_size:
                            continue
                        if max_size > 0 and file_size > max_size:
                            continue
                    except OSError:
                        continue
                    
                    # Check exclude patterns
                    if any(regex.search(filename) for regex in exclude_regexes):
                        continue
                    
                    # Check file name pattern
                    if not file_regex.search(filename):
                        continue
                    
                    files_to_search.append((filename, filepath))
                
                if not recursive:
                    break
            
            print(f"\nFile collection complete:")
            print(f"  Total files matching filename pattern: {len(files_to_search)}")
            try:
                pdf_count = sum(1 for _, fp in files_to_search if fp.lower().endswith('.pdf'))
                docx_count = sum(1 for _, fp in files_to_search if fp.lower().endswith('.docx'))
                self.log(1, f"  Breakdown: {pdf_count} PDF(s), {docx_count} DOCX file(s)")
            except Exception:
                pass
            if len(files_to_search) > 0:
                print(f"  First few files:")
                for filename, filepath in files_to_search[:5]:
                    print(f"    - {filename}")
                if len(files_to_search) > 5:
                    print(f"    ... and {len(files_to_search) - 5} more")
            print("-"*70)
            
            # Process files in parallel using thread pool
            # Use smaller batches for more responsive stopping
            max_workers = min(8, os.cpu_count() or 4)
            total_files = len(files_to_search)
            last_update_time = time.time()
            
            with ThreadPoolExecutor(max_workers=max_workers) as executor:
                # Submit in batches instead of all at once for better stop responsiveness
                batch_size = max_workers * 2
                file_index = 0
                
                while file_index < total_files and not self.stop_search:
                    # Submit batch of files
                    batch_end = min(file_index + batch_size, total_files)
                    futures = []
                    
                    for filename, filepath in files_to_search[file_index:batch_end]:
                        if self.stop_search:
                            break
                        future = executor.submit(
                            self.search_file,
                            filename, filepath, content_regex,
                            context_before, context_after
                        )
                        futures.append(future)
                    
                    # Process this batch
                    for future in as_completed(futures):
                        if self.stop_search:
                            # Cancel any pending futures
                            for f in futures:
                                f.cancel()
                            break
                        
                        result = future.result()
                        if result:
                            filename, filepath, content_matches = result
                            files_found += 1
                            GLib.idle_add(self.add_file_result, filename, filepath, content_matches)
                        
                        files_searched += 1
                        
                        # Update status bar every 3 seconds (throttled)
                        current_time = time.time()
                        if current_time - last_update_time >= 3.0:
                            GLib.idle_add(self.update_status, files_searched, files_found, total_files)
                            last_update_time = current_time
                    
                    file_index = batch_end
            
            # Final update with exact counts
            print("\n" + "="*70)
            print("SEARCH COMPLETE")
            print(f"Files searched: {files_searched}")
            print(f"Files found: {files_found}")
            print("="*70 + "\n")
            GLib.idle_add(self.search_complete, files_found, files_searched)
            
        except Exception as e:
            print(f"\nERROR during search: {e}")
            import traceback
            traceback.print_exc()
            GLib.idle_add(self.show_error, str(e))
            GLib.idle_add(self.search_complete, 0, 0)
    
    def search_file(self, filename, filepath, content_regex, context_before, context_after):
        """Search a single file (called from thread pool)"""
        try:
            # Check if stop was requested
            if self.stop_search:
                return None
                
            content_matches = []
            
            # If no content search, just return the file
            if not content_regex:
                print(f"  ✓ {filename} (filename match only)")
                return (filename, filepath, [])
            
            # Handle PDF files
            if filepath.lower().endswith('.pdf'):
                print(f"  Searching PDF: {filename}")
                pdf_text = self.extract_pdf_text(filepath)
                if pdf_text:
                    lines = pdf_text.split('\n')
                    print(f"    PDF extracted: {len(lines)} lines")
                    for i, line in enumerate(lines):
                        # Check stop every 100 lines
                        if i % 100 == 0 and self.stop_search:
                            return None
                            
                        if content_regex.search(line):
                            start = max(0, i - context_before)
                            end = min(len(lines), i + context_after + 1)
                            context = lines[start:end]
                            content_matches.append((i + 1, start + 1, context))
                    
                    if content_matches:
                        print(f"  ✓ {filename} (found {len(content_matches)} matches in PDF)")
                        return (filename, filepath, content_matches)
                    else:
                        print(f"  ✗ {filename} (PDF extracted but no content matches)")
                else:
                    print(f"  ✗ {filename} (PDF text extraction failed)")
                return None
            
            # Check if binary file
            if self.is_binary_file(filepath):
                return None
            
            # Search text file with streaming (line-by-line)
            try:
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                    # For context lines, we need to keep a buffer
                    line_buffer = []
                    line_num = 0
                    
                    for line in f:
                        # Check stop every 100 lines
                        if line_num % 100 == 0 and self.stop_search:
                            return None
                            
                        line_num += 1
                        line_buffer.append((line_num, line))
                        
                        # Keep buffer size manageable
                        max_buffer = context_before + context_after + 100
                        if len(line_buffer) > max_buffer:
                            line_buffer.pop(0)
                        
                        if content_regex.search(line):
                            # Find this line's position in buffer
                            current_pos = len(line_buffer) - 1
                            
                            # Get context
                            start_pos = max(0, current_pos - context_before)
                            end_pos = min(len(line_buffer), current_pos + context_after + 1)
                            
                            context_lines = [l[1] for l in line_buffer[start_pos:end_pos]]
                            start_line_num = line_buffer[start_pos][0]
                            
                            content_matches.append((line_num, start_line_num, context_lines))
                
                if content_matches:
                    return (filename, filepath, content_matches)
                    
            except (OSError, UnicodeDecodeError):
                return None
            
        except Exception:
            return None
        
        return None
    
    def search_with_ripgrep(self, search_dir, file_regex, content_pattern,
                           case_sensitive, context_before, context_after,
                           exclude_regexes, include_hidden, min_size, max_size):
        """Use ripgrep for faster content searching"""
        try:
            # Build ripgrep command
            cmd = ['rg', '--line-number', '--with-filename', '--no-heading']
            
            # Case sensitivity
            if not case_sensitive:
                cmd.append('--ignore-case')
            
            # Context lines
            if context_before > 0:
                cmd.extend(['--before-context', str(context_before)])
            if context_after > 0:
                cmd.extend(['--after-context', str(context_after)])
            
            # Hidden files
            if include_hidden:
                cmd.append('--hidden')
            
            # Add pattern and directory
            cmd.append(content_pattern)
            cmd.append(search_dir)
            
            self.log(2, f"Ripgrep command: {' '.join(cmd)}")

            # Run ripgrep - capture as bytes to avoid encoding issues
            result = subprocess.run(
                cmd,
                capture_output=True,
                timeout=300  # 5 minute timeout
            )
            
            self.log(2, f"Ripgrep return code: {result.returncode}")
            if result.stderr:
                try:
                    self.log(2, f"Ripgrep stderr: {result.stderr.decode('utf-8', errors='replace')}")
                except Exception:
                    self.log(2, "Ripgrep stderr: (unable to decode)")

            # Decode output with error handling for non-UTF-8 content
            try:
                output = result.stdout.decode('utf-8')
            except UnicodeDecodeError:
                # Try with latin-1 which accepts all byte values
                output = result.stdout.decode('latin-1', errors='replace')
            
            # Parse ripgrep output
            files_found = 0
            current_file = None
            current_matches = []
            current_context = []
            match_line_num = 0
            context_start_line = 0
            
            for line in output.split('\n'):
                if not line:
                    continue
                
                # Parse ripgrep output format: filepath:line_num:content
                parts = line.split(':', 2)
                if len(parts) >= 3:
                    filepath = parts[0]
                    try:
                        line_num = int(parts[1])
                    except:
                        continue
                    content = parts[2] if len(parts) > 2 else ''
                    
                    filename = os.path.basename(filepath)
                    
                    # Check if this file matches our file pattern and filters
                    if not file_regex.search(filename):
                        continue
                    
                    if any(regex.search(filename) for regex in exclude_regexes):
                        continue
                    
                    # Check file size
                    try:
                        file_size = os.path.getsize(filepath)
                        if min_size > 0 and file_size < min_size:
                            continue
                        if max_size > 0 and file_size > max_size:
                            continue
                    except:
                        continue
                    
                    # Group matches by file
                    if filepath != current_file:
                        # Add previous file if it had matches
                        if current_file and current_matches:
                            GLib.idle_add(self.add_file_result, 
                                        os.path.basename(current_file),
                                        current_file,
                                        current_matches)
                            files_found += 1
                        
                        current_file = filepath
                        current_matches = []
                        current_context = [content]
                        match_line_num = line_num
                        context_start_line = line_num
                    else:
                        current_context.append(content)
                        
                        # Check if this is a new match (gap in line numbers)
                        if line_num > context_start_line + len(current_context):
                            # Save previous match
                            current_matches.append((match_line_num, context_start_line, current_context))
                            current_context = [content]
                            match_line_num = line_num
                            context_start_line = line_num
            
            # Add last file
            if current_file and current_matches:
                GLib.idle_add(self.add_file_result,
                            os.path.basename(current_file),
                            current_file,
                            current_matches)
                files_found += 1
            
            return files_found
            
        except subprocess.TimeoutExpired:
            # Search took too long, fall back to Python
            GLib.idle_add(self.statusbar.push, self.context_id, 
                         "Search timeout - switching to Python search mode")
            return -1  # Signal to fall back to Python search
        except Exception as e:
            # Any other error - silently fall back to Python search
            # Don't show error dialog, just update status bar
            GLib.idle_add(self.statusbar.push, self.context_id, 
                         "Using Python search mode")
            return -1  # Signal to fall back to Python search
    
    def add_file_result(self, filename, filepath, content_matches):
        """Add a file to the results (called from main thread)"""
        # Get file metadata
        try:
            file_size = os.path.getsize(filepath)
            size_human = self.format_file_size(file_size)
            modified_date = self.get_file_modified_date(filepath)
        except:
            file_size = 0
            size_human = "Unknown"
            modified_date = "Unknown"
        
        # Store content matches with the file data - filename, full path, modified date, size (bytes), size (human)
        iter = self.file_store.append([filename, filepath, modified_date, file_size, size_human])
        
        # Store matches for later retrieval
        if not hasattr(self, 'file_matches'):
            self.file_matches = {}
        self.file_matches[filepath] = content_matches
        
        # Add to overview
        self.add_to_overview(filename, filepath, modified_date, content_matches)
        
        return False
    
    def add_to_overview(self, filename, filepath, modified_date, content_matches):
        """Add file to results overview ListBox"""
        # Create a row for the ListBox
        row = Gtk.ListBoxRow()
        row.filepath = filepath  # Store filepath as attribute
        
        # Create vertical box for content (no EventBox - let ListBox handle selection)
        content_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=3)
        content_box.set_margin_start(15)
        content_box.set_margin_end(15)
        content_box.set_margin_top(10)
        content_box.set_margin_bottom(10)
        row.add(content_box)
        
        # Filename label (bold, larger) - store reference for selection styling
        filename_label = Gtk.Label()
        filename_label.set_markup(f"<span size='large' weight='bold'>{filename}</span>")
        filename_label.set_xalign(0.5)
        row.filename_label = filename_label  # Store for later access
        row.filename_text = filename  # Store original text
        content_box.pack_start(filename_label, False, False, 0)
        
        # Modified date label (gray)
        date_label = Gtk.Label()
        date_label.set_markup(f"<span foreground='gray'>Modified: {modified_date}</span>")
        date_label.set_xalign(0.5)
        content_box.pack_start(date_label, False, False, 0)
        
        # Add context from first match if content search was performed
        if content_matches and len(content_matches) > 0:
            match_line, start_line, context = content_matches[0]
            
            # Get content pattern for highlighting
            content_pattern = self.content_combo.get_child().get_text()
            flags = 0 if self.case_check.get_active() else re.IGNORECASE
            
            # Add spacing
            spacer = Gtk.Label(label="")
            content_box.pack_start(spacer, False, False, 0)
            
            try:
                pattern = re.compile(content_pattern, flags)
                
                # Create text for context lines with highlighting
                for i, line in enumerate(context[:3]):  # Show first 3 lines of context
                    line_num = start_line + i
                    
                    # Create label for line number + content
                    line_label = Gtk.Label()
                    line_label.set_xalign(0)
                    line_label.set_line_wrap(True)
                    line_label.set_line_wrap_mode(Pango.WrapMode.WORD)  # Wrap at word boundaries
                    line_label.set_selectable(False)
                    
                    # Build markup with highlighting
                    if line_num == match_line:
                        # This is the match line - highlight matches
                        line_markup = f"<span foreground='#5DADE2'>Line {line_num}:</span> "
                        
                        last_end = 0
                        for match in pattern.finditer(line):
                            # Add text before match
                            if match.start() > last_end:
                                text_before = GLib.markup_escape_text(line[last_end:match.start()])
                                line_markup += text_before
                            
                            # Add highlighted match
                            match_text = GLib.markup_escape_text(match.group())
                            line_markup += f"<span background='#5DADE2' foreground='white' weight='bold'>{match_text}</span>"
                            last_end = match.end()
                        
                        # Add remaining text
                        if last_end < len(line):
                            text_after = GLib.markup_escape_text(line[last_end:])
                            line_markup += text_after
                        
                        line_label.set_markup(line_markup)
                    else:
                        # Context line - no highlighting
                        escaped_line = GLib.markup_escape_text(line)
                        line_label.set_markup(f"<span foreground='#5DADE2'>Line {line_num}:</span> {escaped_line}")
                    
                    content_box.pack_start(line_label, False, False, 0)
            except Exception as e:
                # If regex fails or markup fails, show plain text
                for i, line in enumerate(context[:3]):
                    line_num = start_line + i
                    line_label = Gtk.Label(label=f"Line {line_num}: {line}")
                    line_label.set_xalign(0)
                    line_label.set_line_wrap(True)
                    content_box.pack_start(line_label, False, False, 0)
        
        # Add horizontal separator
        separator = Gtk.Separator(orientation=Gtk.Orientation.HORIZONTAL)
        separator.set_margin_top(10)
        content_box.pack_start(separator, False, False, 0)
        
        # Add row to ListBox
        self.overview_listbox.add(row)
        row.show_all()
        
        return False
    
    def on_overview_row_activated(self, listbox, row):
        """Handle double-click on overview row (row-activated signal)"""
        if hasattr(row, 'filepath'):
            self.open_file_with_default(row.filepath)
    
    def on_overview_row_selected(self, listbox, row):
        """Handle selection change - highlight just the filename"""
        # Unhighlight all rows first
        for child_row in listbox.get_children():
            if hasattr(child_row, 'filename_label') and hasattr(child_row, 'filename_text'):
                # Reset to normal (bold but no background)
                child_row.filename_label.set_markup(
                    f"<span size='large' weight='bold'>{child_row.filename_text}</span>"
                )
        
        # Highlight selected row's filename
        if row and hasattr(row, 'filename_label') and hasattr(row, 'filename_text'):
            row.filename_label.set_markup(
                f"<span size='large' weight='bold' background='#5DADE2' foreground='white'>{row.filename_text}</span>"
            )
    
    def on_overview_listbox_button_press(self, listbox, event):
        """Handle button press on ListBox for right-click menu"""
        # Only handle right-click
        if event.button == 3:
            # Get the row at the click position
            row = listbox.get_row_at_y(int(event.y))
            if row and hasattr(row, 'filepath'):
                filepath = row.filepath
                
                menu = Gtk.Menu()
                
                # Open in default program
                open_default_item = Gtk.MenuItem(label="Open in default program")
                open_default_item.connect("activate", self.on_open_in_default, filepath)
                menu.append(open_default_item)
                
                # Open in... (choose application)
                open_with_item = Gtk.MenuItem(label="Open with...")
                open_with_item.connect("activate", self.on_open_with_dialog, filepath)
                menu.append(open_with_item)
                
                menu.append(Gtk.SeparatorMenuItem())
                
                # Open in file manager
                show_item = Gtk.MenuItem(label="Show in file manager")
                show_item.connect("activate", self.on_show_in_file_manager, filepath)
                menu.append(show_item)
                
                # Copy path
                copy_item = Gtk.MenuItem(label="Copy path")
                copy_item.connect("activate", self.on_copy_path, filepath)
                menu.append(copy_item)
                
                menu.append(Gtk.SeparatorMenuItem())
                
                # Add to favorites
                add_fav_item = Gtk.MenuItem(label="Add to Favorites")
                add_fav_item.connect("activate", self.on_add_to_favorites, filepath)
                menu.append(add_fav_item)
                
                menu.append(Gtk.SeparatorMenuItem())
                
                # Delete file
                delete_item = Gtk.MenuItem(label="Delete file")
                delete_item.connect("activate", self.on_delete_file, filepath)
                menu.append(delete_item)
                
                menu.show_all()
                menu.popup_at_pointer(event)
                return True
        
        # Let ListBox handle other events (selection)
        return False
    
    def on_file_tree_key_press(self, tree_view, event):
        """Handle key press on file tree"""
        # Check if Enter key was pressed
        if event.keyval == 65293:  # Enter key
            selection = tree_view.get_selection()
            model, iter = selection.get_selected()
            
            if iter is not None:
                filepath = model[iter][1]
                self.open_file_with_default(filepath)
                return True
        return False
    
    def on_file_tree_double_click(self, tree_view, path, column):
        """Handle double-click on file tree"""
        model = tree_view.get_model()
        iter = model.get_iter(path)
        if iter is not None:
            filepath = model[iter][1]
            self.open_file_with_default(filepath)
    
    def on_overview_key_press(self, text_view, event):
        """Handle key press on overview text view"""
        # Check if Enter key was pressed
        if event.keyval == 65293:  # Enter key
            # Get the current cursor position
            buffer = text_view.get_buffer()
            cursor_mark = buffer.get_insert()
            cursor_iter = buffer.get_iter_at_mark(cursor_mark)
            offset = cursor_iter.get_offset()
            
            # Find the file at this position
            if hasattr(self, 'overview_file_map'):
                # Find the closest file entry before this offset
                filepath = None
                closest_offset = -1
                for file_offset, file_path in self.overview_file_map.items():
                    if file_offset <= offset and file_offset > closest_offset:
                        closest_offset = file_offset
                        filepath = file_path
                
                if filepath:
                    self.open_file_with_default(filepath)
                    return True
        return False
    
    def open_file_with_default(self, filepath):
        """Open file with default application using xdg-open"""
        try:
            import subprocess
            subprocess.Popen(['xdg-open', filepath], 
                           stdout=subprocess.DEVNULL, 
                           stderr=subprocess.DEVNULL)
        except Exception as e:
            dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="Failed to open file"
            )
            dialog.format_secondary_text(f"Could not open {filepath}: {str(e)}")
            dialog.run()
            dialog.destroy()
    
    def update_status(self, searched, found, total):
        """Update status bar (called from main thread)"""
        self.statusbar.push(
            self.context_id,
            f"Searching... {searched}/{total} files searched, {found} found"
        )
        return False
    
    def search_complete(self, files_found, files_searched):
        """Handle search completion (called from main thread)"""
        self.search_button.set_sensitive(True)
        self.stop_button.set_sensitive(False)
        
        # If files_searched == files_found, we don't have accurate total (e.g., ripgrep)
        # Show simpler message in that case
        if files_searched == files_found and files_found > 0:
            # Ripgrep or similar - we don't know total searched
            if self.stop_search:
                self.statusbar.push(self.context_id, f"Search stopped. Found {files_found} file(s).")
            else:
                self.statusbar.push(self.context_id, f"Search complete. Found {files_found} file(s).")
        else:
            # Normal case - we know total searched
            if self.stop_search:
                self.statusbar.push(self.context_id, f"Search stopped. Found {files_found} files out of {files_searched} searched.")
            else:
                self.statusbar.push(self.context_id, f"Search complete. Found {files_found} files out of {files_searched} searched.")
        
        return False
    
    def show_regex_error(self, error_msg):
        """Show regex error dialog"""
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.ERROR,
            buttons=Gtk.ButtonsType.OK,
            text="Invalid Regular Expression"
        )
        dialog.format_secondary_text(error_msg)
        dialog.run()
        dialog.destroy()
        return False
    
    def show_error(self, error_msg):
        """Show general error dialog"""
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.ERROR,
            buttons=Gtk.ButtonsType.OK,
            text="Search Error"
        )
        dialog.format_secondary_text(error_msg)
        dialog.run()
        dialog.destroy()
        return False
    
    def on_file_selected(self, tree_view):
        """Handle file selection in results"""
        selection = tree_view.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return
        
        filepath = model[iter][1]
        
        # Display content matches
        self.content_buffer.set_text("")
        
        if not hasattr(self, 'file_matches') or filepath not in self.file_matches:
            # No content matches, just show file path
            self.content_buffer.set_text(f"File: {filepath}\n\nNo content search performed or no matches found.")
            return
        
        matches = self.file_matches[filepath]
        
        if not matches:
            self.content_buffer.set_text(f"File: {filepath}\n\nNo content matches (filename only match).")
            return
        
        # Display matches with highlighting
        iter = self.content_buffer.get_start_iter()
        
        for match_line, start_line, context in matches:
            # Add separator
            if iter.get_offset() > 0:
                self.content_buffer.insert(iter, "\n" + "-" * 60 + "\n")
            
            # Add each line with line numbers
            for i, line in enumerate(context):
                line_num = start_line + i
                current_line = match_line
                
                # Insert line number
                line_start = iter.get_offset()
                self.content_buffer.insert(iter, f"{line_num:4d}: ")
                line_end = self.content_buffer.get_iter_at_offset(line_start)
                line_num_end = iter.copy()
                self.content_buffer.apply_tag(self.line_number_tag, line_end, line_num_end)
                
                # Insert line content with highlighting if it's the match line
                if line_num == current_line:
                    # Highlight matches in this line
                    content_pattern = self.content_combo.get_child().get_text()
                    flags = 0 if self.case_check.get_active() else re.IGNORECASE
                    try:
                        pattern = re.compile(content_pattern, flags)
                        last_end = 0
                        
                        for match in pattern.finditer(line):
                            # Insert text before match
                            if match.start() > last_end:
                                self.content_buffer.insert(iter, line[last_end:match.start()])
                            
                            # Insert and highlight match
                            match_start = iter.get_offset()
                            self.content_buffer.insert(iter, match.group())
                            match_end = iter.copy()
                            match_start_iter = self.content_buffer.get_iter_at_offset(match_start)
                            self.content_buffer.apply_tag(self.match_tag, match_start_iter, match_end)
                            
                            last_end = match.end()
                        
                        # Insert remaining text
                        if last_end < len(line):
                            self.content_buffer.insert(iter, line[last_end:])
                    except:
                        self.content_buffer.insert(iter, line)
                else:
                    self.content_buffer.insert(iter, line)
    
    def on_file_right_click(self, tree_view, event):
        """Handle right-click on file in results"""
        if event.button != 3:  # Right click
            return False
        
        selection = tree_view.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return False
        
        filepath = model[iter][1]
        
        menu = Gtk.Menu()
        
        # Open in default program
        open_default_item = Gtk.MenuItem(label="Open in default program")
        open_default_item.connect("activate", self.on_open_in_default, filepath)
        menu.append(open_default_item)
        
        # Open in... (choose application)
        open_with_item = Gtk.MenuItem(label="Open with...")
        open_with_item.connect("activate", self.on_open_with_dialog, filepath)
        menu.append(open_with_item)
        
        menu.append(Gtk.SeparatorMenuItem())
        
        # Open in file manager
        show_item = Gtk.MenuItem(label="Show in file manager")
        show_item.connect("activate", self.on_show_in_file_manager, filepath)
        menu.append(show_item)
        
        menu.append(Gtk.SeparatorMenuItem())
        
        # Copy path
        copy_item = Gtk.MenuItem(label="Copy path")
        copy_item.connect("activate", self.on_copy_path, filepath)
        menu.append(copy_item)
        
        menu.append(Gtk.SeparatorMenuItem())
        
        # Add to Favorites
        add_fav_item = Gtk.MenuItem(label="Add to Favorites")
        add_fav_item.connect("activate", self.on_add_to_favorites, filepath)
        menu.append(add_fav_item)
        
        menu.append(Gtk.SeparatorMenuItem())
        
        # Delete file
        delete_item = Gtk.MenuItem(label="Delete file")
        delete_item.connect("activate", self.on_delete_file, filepath, iter)
        menu.append(delete_item)
        
        menu.show_all()
        menu.popup(None, None, None, None, event.button, event.time)
        
        return True
    
    def on_open_in_default(self, menu_item, filepath):
        """Open file in default application"""
        self.open_file_with_default(filepath)
    
    def on_open_with_dialog(self, menu_item, filepath):
        """Show dialog to choose application to open file with"""
        import subprocess
        import mimetypes
        
        # Get MIME type
        mime_type, _ = mimetypes.guess_type(filepath)
        if mime_type is None:
            mime_type = 'application/octet-stream'
        
        # Create dialog to choose application
        dialog = Gtk.Dialog(
            title="Open With",
            transient_for=self,
            flags=0
        )
        dialog.add_buttons(
            Gtk.STOCK_CANCEL, Gtk.ResponseType.CANCEL,
            Gtk.STOCK_OK, Gtk.ResponseType.OK
        )
        dialog.set_default_size(450, 400)
        
        content_area = dialog.get_content_area()
        content_area.set_spacing(10)
        content_area.set_margin_start(10)
        content_area.set_margin_end(10)
        content_area.set_margin_top(10)
        content_area.set_margin_bottom(10)
        
        label = Gtk.Label(label=f"Choose application to open:\n{os.path.basename(filepath)}")
        content_area.pack_start(label, False, False, 0)
        
        # Create list of applications
        scrolled = Gtk.ScrolledWindow()
        scrolled.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        
        liststore = Gtk.ListStore(str, str)
        
        # Add default applications
        apps = [
            ("Default (xdg-open)", "xdg-open"),
            ("gedit", "gedit"),
            ("nano", "nano"),
            ("vim", "vim"),
            ("kate", "kate"),
            ("mousepad", "mousepad"),
            ("leafpad", "leafpad"),
            ("pluma", "pluma"),
            ("VSCode", "code"),
            ("Sublime Text", "subl"),
        ]
        
        # Check which ones are available
        available_apps = []
        for name, cmd in apps:
            try:
                result = subprocess.run(['which', cmd.split()[0]], 
                                      capture_output=True, 
                                      text=True)
                if result.returncode == 0:
                    available_apps.append((name, cmd))
            except:
                pass
        
        # Load custom programs from config
        custom_programs = self.get_custom_programs()
        for name, cmd in custom_programs:
            available_apps.append((f"{name} (custom)", cmd))
        
        # Add to liststore
        for name, cmd in available_apps:
            liststore.append([name, cmd])
        
        treeview = Gtk.TreeView(model=liststore)
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Application", renderer, text=0)
        treeview.append_column(column)
        
        scrolled.add(treeview)
        content_area.pack_start(scrolled, True, True, 0)
        
        # Add custom program section
        custom_frame = Gtk.Frame(label="Add Custom Program")
        custom_frame.set_margin_top(10)
        custom_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        custom_box.set_margin_start(10)
        custom_box.set_margin_end(10)
        custom_box.set_margin_top(10)
        custom_box.set_margin_bottom(10)
        
        name_label = Gtk.Label(label="Name:")
        custom_name_entry = Gtk.Entry()
        custom_name_entry.set_placeholder_text("e.g., My Editor")
        custom_name_entry.set_width_chars(12)
        
        cmd_label = Gtk.Label(label="Command:")
        custom_cmd_entry = Gtk.Entry()
        custom_cmd_entry.set_placeholder_text("e.g., emacs")
        custom_cmd_entry.set_width_chars(15)
        
        add_button = Gtk.Button(label="Add")
        
        def on_add_custom(button):
            name = custom_name_entry.get_text().strip()
            cmd = custom_cmd_entry.get_text().strip()
            if name and cmd:
                # Save to config
                self.save_custom_program(name, cmd)
                # Add to list
                liststore.append([f"{name} (custom)", cmd])
                # Clear entries
                custom_name_entry.set_text("")
                custom_cmd_entry.set_text("")
        
        add_button.connect("clicked", on_add_custom)
        
        custom_box.pack_start(name_label, False, False, 0)
        custom_box.pack_start(custom_name_entry, False, False, 0)
        custom_box.pack_start(cmd_label, False, False, 0)
        custom_box.pack_start(custom_cmd_entry, True, True, 0)
        custom_box.pack_start(add_button, False, False, 0)
        
        custom_frame.add(custom_box)
        content_area.pack_start(custom_frame, False, False, 0)
        
        dialog.show_all()
        response = dialog.run()
        
        if response == Gtk.ResponseType.OK:
            selection = treeview.get_selection()
            model, iter = selection.get_selected()
            if iter is not None:
                cmd = model[iter][1]
                try:
                    subprocess.Popen([cmd, filepath],
                                   stdout=subprocess.DEVNULL,
                                   stderr=subprocess.DEVNULL)
                except Exception as e:
                    error_dialog = Gtk.MessageDialog(
                        transient_for=self,
                        flags=0,
                        message_type=Gtk.MessageType.ERROR,
                        buttons=Gtk.ButtonsType.OK,
                        text="Failed to open file"
                    )
                    error_dialog.format_secondary_text(f"Could not open with {cmd}: {str(e)}")
                    error_dialog.run()
                    error_dialog.destroy()
        
        dialog.destroy()
    
    def on_show_in_file_manager(self, menu_item, filepath):
        """Show file in file manager"""
        try:
            directory = os.path.dirname(filepath)
            os.system(f"xdg-open '{directory}' &")
        except Exception as e:
            self.show_error(f"Failed to open file manager: {str(e)}")
    
    def on_copy_path(self, menu_item, filepath):
        """Copy file path to clipboard"""
        clipboard = Gtk.Clipboard.get(Gtk.gdk.SELECTION_CLIPBOARD)
        clipboard.set_text(filepath, -1)
        clipboard.store()
        self.statusbar.push(self.context_id, "Path copied to clipboard")
    
    def on_delete_file(self, menu_item, filepath, tree_iter):
        """Delete the selected file"""
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.WARNING,
            buttons=Gtk.ButtonsType.YES_NO,
            text="Delete File?"
        )
        dialog.format_secondary_text(
            f"Are you sure you want to delete:\n{filepath}\n\nThis action cannot be undone."
        )
        
        response = dialog.run()
        dialog.destroy()
        
        if response == Gtk.ResponseType.YES:
            try:
                os.remove(filepath)
                self.file_store.remove(tree_iter)
                if hasattr(self, 'file_matches') and filepath in self.file_matches:
                    del self.file_matches[filepath]
                self.statusbar.push(self.context_id, f"Deleted: {filepath}")
            except Exception as e:
                self.show_error(f"Failed to delete file: {str(e)}")
    
    def on_favorites_right_click(self, tree_view, event):
        """Handle right-click on favorites"""
        if event.button != 3:  # Right click
            return False
        
        selection = tree_view.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return False
        
        is_category = model[iter][0]
        
        # Only show file menu for files, not categories
        if is_category:
            return False
        
        filepath = model[iter][2]  # Column 2 is filepath for files
        
        if not filepath:  # Safety check
            return False
        
        menu = Gtk.Menu()
        
        # Open in default program
        open_default_item = Gtk.MenuItem(label="Open in default program")
        open_default_item.connect("activate", self.on_open_in_default, filepath)
        menu.append(open_default_item)
        
        # Open in... (choose application)
        open_with_item = Gtk.MenuItem(label="Open with...")
        open_with_item.connect("activate", self.on_open_with_dialog, filepath)
        menu.append(open_with_item)
        
        menu.append(Gtk.SeparatorMenuItem())
        
        # Open in file manager
        show_item = Gtk.MenuItem(label="Show in file manager")
        show_item.connect("activate", self.on_show_in_file_manager, filepath)
        menu.append(show_item)
        
        menu.append(Gtk.SeparatorMenuItem())
        
        # Copy path
        copy_item = Gtk.MenuItem(label="Copy path")
        copy_item.connect("activate", self.on_copy_path, filepath)
        menu.append(copy_item)
        
        menu.append(Gtk.SeparatorMenuItem())
        
        # Remove from Favorites
        remove_fav_item = Gtk.MenuItem(label="Remove from Favorites")
        remove_fav_item.connect("activate", self.on_remove_from_favorites, iter)
        menu.append(remove_fav_item)
        
        menu.show_all()
        menu.popup(None, None, None, None, event.button, event.time)
        
        return True
    
    def on_favorites_key_press(self, tree_view, event):
        """Handle key press on favorites tree"""
        # Check if Alt key is pressed
        is_alt = event.state & Gdk.ModifierType.MOD1_MASK
        
        # Alt+Up or Alt+Down to reorder
        if is_alt and event.keyval == Gdk.KEY_Up:
            self.move_favorite_up()
            return True
        elif is_alt and event.keyval == Gdk.KEY_Down:
            self.move_favorite_down()
            return True
        
        # Check if Enter key was pressed
        if event.keyval == 65293:  # Enter key
            selection = tree_view.get_selection()
            model, iter = selection.get_selected()
            
            if iter is not None:
                is_category = model[iter][0]
                if not is_category:  # Only open files, not categories
                    filepath = model[iter][2]  # Column 2 is filepath
                    if filepath:
                        self.open_file_with_default(filepath)
                        return True
        return False
    
    def on_favorites_double_click(self, tree_view, path, column):
        """Handle double-click on favorites tree"""
        model = tree_view.get_model()
        iter = model.get_iter(path)
        if iter is not None:
            is_category = model[iter][0]
            if not is_category:  # Only open files, not categories
                filepath = model[iter][2]  # Column 2 is filepath for files
                if filepath:  # Make sure it's not empty
                    self.open_file_with_default(filepath)
    
    def on_remove_from_favorites(self, menu_item, tree_iter):
        """Remove a file from favorites"""
        # tree_iter is the direct TreeStore iterator (no sorted model anymore)
        filename = self.favorites_store[tree_iter][1]  # Column 1 is name/filename
        
        # Remove from store
        self.favorites_store.remove(tree_iter)
        
        # Save updated favorites
        self.save_favorites()
        
        self.statusbar.push(self.context_id, f"Removed from favorites: {filename}")
    
    # Category Management Methods
    def on_add_category(self, button):
        """Add a new category"""
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL,
            text="Add Category"
        )
        dialog.format_secondary_text("Enter category name:")
        
        content_area = dialog.get_content_area()
        entry = Gtk.Entry()
        entry.set_placeholder_text("Category name")
        entry.connect("activate", lambda e: dialog.response(Gtk.ResponseType.OK))
        content_area.pack_start(entry, False, False, 0)
        
        dialog.show_all()
        response = dialog.run()
        name = entry.get_text().strip()
        dialog.destroy()
        
        if response == Gtk.ResponseType.OK and name:
            # Add category node
            self.favorites_store.append(None, [True, name, "", "", "", 0, "", ""])
            self.save_favorites()
            self.statusbar.push(self.context_id, f"Added category: {name}")
    
    def on_rename_category(self, button):
        """Rename selected category"""
        selection = self.favorites_tree.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return
        
        is_category = model[iter][0]
        if not is_category:
            self.statusbar.push(self.context_id, "Please select a category to rename")
            return
        
        old_name = model[iter][1]
        
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL,
            text="Rename Category"
        )
        dialog.format_secondary_text(f"Rename '{old_name}' to:")
        
        content_area = dialog.get_content_area()
        entry = Gtk.Entry()
        entry.set_text(old_name)
        entry.connect("activate", lambda e: dialog.response(Gtk.ResponseType.OK))
        content_area.pack_start(entry, False, False, 0)
        
        dialog.show_all()
        response = dialog.run()
        new_name = entry.get_text().strip()
        dialog.destroy()
        
        if response == Gtk.ResponseType.OK and new_name and new_name != old_name:
            # Rename category
            model[iter][1] = new_name
            
            # Update all children's category field
            child_iter = model.iter_children(iter)
            while child_iter is not None:
                model[child_iter][7] = new_name  # category column
                child_iter = model.iter_next(child_iter)
            
            self.save_favorites()
            self.statusbar.push(self.context_id, f"Renamed category to: {new_name}")
    
    def on_move_to_category(self, button):
        """Move selected file to a different category"""
        selection = self.favorites_tree.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return
        
        is_category = model[iter][0]
        if is_category:
            self.statusbar.push(self.context_id, "Please select a file to move")
            return
        
        # Get current file data
        file_data = [model[iter][i] for i in range(8)]
        
        # Get list of categories
        categories = []
        cat_iter = model.get_iter_first()
        while cat_iter is not None:
            if model[cat_iter][0]:  # is_category
                categories.append(model[cat_iter][1])
            cat_iter = model.iter_next(cat_iter)
        
        if not categories:
            self.statusbar.push(self.context_id, "No categories available")
            return
        
        # Show category selection dialog
        dialog = Gtk.Dialog(
            title="Move to Category",
            transient_for=self,
            flags=0
        )
        dialog.add_buttons(
            Gtk.STOCK_CANCEL, Gtk.ResponseType.CANCEL,
            Gtk.STOCK_OK, Gtk.ResponseType.OK
        )
        
        content_area = dialog.get_content_area()
        content_area.set_spacing(10)
        content_area.set_margin_start(10)
        content_area.set_margin_end(10)
        content_area.set_margin_top(10)
        content_area.set_margin_bottom(10)
        
        label = Gtk.Label(label=f"Move '{file_data[1]}' to category:")
        content_area.pack_start(label, False, False, 0)
        
        combo = Gtk.ComboBoxText()
        for cat in categories:
            combo.append_text(cat)
        combo.set_active(0)
        content_area.pack_start(combo, False, False, 0)
        
        dialog.show_all()
        response = dialog.run()
        target_category = combo.get_active_text()
        dialog.destroy()
        
        if response == Gtk.ResponseType.OK and target_category:
            # Find target category iter
            target_iter = None
            cat_iter = model.get_iter_first()
            while cat_iter is not None:
                if model[cat_iter][0] and model[cat_iter][1] == target_category:
                    target_iter = cat_iter
                    break
                cat_iter = model.iter_next(cat_iter)
            
            if target_iter:
                # Remove from current location
                model.remove(iter)
                
                # Add to new category
                file_data[7] = target_category  # Update category name
                model.append(target_iter, file_data)
                
                # Expand target category
                path = model.get_path(target_iter)
                self.favorites_tree.expand_row(path, False)
                
                self.save_favorites()
                self.statusbar.push(self.context_id, f"Moved to category: {target_category}")
    
    def on_delete_category(self, button):
        """Delete selected category (and optionally its files)"""
        selection = self.favorites_tree.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return
        
        is_category = model[iter][0]
        if not is_category:
            self.statusbar.push(self.context_id, "Please select a category to delete")
            return
        
        category_name = model[iter][1]
        
        # Count files in category
        file_count = model.iter_n_children(iter)
        
        if file_count > 0:
            dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.WARNING,
                buttons=Gtk.ButtonsType.YES_NO,
                text="Delete Category"
            )
            dialog.format_secondary_text(
                f"Category '{category_name}' contains {file_count} file(s).\n\n"
                f"Move files to 'Uncategorized' before deleting?\n\n"
                f"Yes = Move files and delete category\n"
                f"No = Delete category and all files in it"
            )
            response = dialog.run()
            dialog.destroy()
            
            if response == Gtk.ResponseType.YES:
                # Move files to Uncategorized
                # Find or create Uncategorized category
                uncategorized_iter = None
                cat_iter = model.get_iter_first()
                while cat_iter is not None:
                    if model[cat_iter][0] and model[cat_iter][1] == "Uncategorized":
                        uncategorized_iter = cat_iter
                        break
                    cat_iter = model.iter_next(cat_iter)
                
                if uncategorized_iter is None:
                    uncategorized_iter = model.append(None, [True, "Uncategorized", "", "", "", 0, "", ""])
                
                # Move all files
                child_iter = model.iter_children(iter)
                files_to_move = []
                while child_iter is not None:
                    file_data = [model[child_iter][i] for i in range(8)]
                    files_to_move.append(file_data)
                    child_iter = model.iter_next(child_iter)
                
                for file_data in files_to_move:
                    file_data[7] = "Uncategorized"
                    model.append(uncategorized_iter, file_data)
        
        # Delete the category
        model.remove(iter)
        self.save_favorites()
        self.statusbar.push(self.context_id, f"Deleted category: {category_name}")
    
    def move_favorite_up(self):
        """Move selected favorite up in the list (Alt+Up)"""
        selection = self.favorites_tree.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return
        
        # Get the path and check if we can move up
        path = model.get_path(iter)
        
        # Can't move if it's the first item or if indices would be invalid
        if path.get_indices()[0] == 0:
            return
        
        # Get previous path
        prev_path = Gtk.TreePath.new_from_string(str(path))
        if prev_path.prev():
            prev_iter = model.get_iter(prev_path)
            # Swap the two rows
            model.swap(iter, prev_iter)
            self.save_favorites()
    
    def move_favorite_down(self):
        """Move selected favorite down in the list (Alt+Down)"""
        selection = self.favorites_tree.get_selection()
        model, iter = selection.get_selected()
        
        if iter is None:
            return
        
        # Get next iter
        next_iter = model.iter_next(iter)
        
        if next_iter is not None:
            # Swap the two rows
            model.swap(iter, next_iter)
            self.save_favorites()
    
    def on_favorites_reordered(self, model, path):
        """Called when favorites are reordered via drag-and-drop"""
        # Save the new order
        self.save_favorites()
        return False


def main():
    app = SearchBoar()
    app.connect("destroy", Gtk.main_quit)
    app.show_all()
    Gtk.main()


if __name__ == "__main__":
    main()
