#!/usr/bin/env python3
"""
Network File Search - GUI Version
GTK-based graphical interface for searching files across network shares
"""

import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, GLib, Pango
import subprocess
import threading
import ipaddress
import re
import tempfile
import shutil
import os
import datetime
import pwd
from pathlib import Path

class NetworkFileSearchGUI(Gtk.Window):
    def __init__(self):
        super().__init__(title="LanSearch")
        self.set_default_size(950, 700)  # Narrower for compact 3-column layout
        self.set_border_width(10)
        
        self.temp_dirs = []
        self.search_running = False
        self.search_thread = None
        
        # Create main container
        vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        self.add(vbox)
        
        # Top section - THREE columns
        top_hbox = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        
        # COLUMN 1: Pattern Builder
        builder_frame = Gtk.Frame(label="Pattern Builder")
        builder_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=3)
        builder_box.set_border_width(5)
        
        # File type selection
        filetype_label = Gtk.Label()
        filetype_label.set_markup("<b>File Type:</b>")
        filetype_label.set_xalign(0)
        builder_box.pack_start(filetype_label, False, False, 0)
        
        # First row of file types
        filetype_button_box1 = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=3)
        
        # Second row of file types
        filetype_button_box2 = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=3)
        
        self.filetype_group = []
        
        all_radio = Gtk.RadioButton(label="All")
        all_radio.set_active(True)
        all_radio.connect("toggled", lambda x: self.update_pattern_display() if x.get_active() else None)
        all_radio.file_pattern = "*"
        self.filetype_group.append(all_radio)
        
        images_radio = Gtk.RadioButton.new_with_label_from_widget(all_radio, "Images")
        images_radio.connect("toggled", lambda x: self.update_pattern_display() if x.get_active() else None)
        images_radio.file_pattern = "*.{jpg,jpeg,png,gif,bmp,webp,tiff,svg}"
        self.filetype_group.append(images_radio)
        
        pdf_radio = Gtk.RadioButton.new_with_label_from_widget(all_radio, "PDFs")
        pdf_radio.connect("toggled", lambda x: self.update_pattern_display() if x.get_active() else None)
        pdf_radio.file_pattern = "*.pdf"
        self.filetype_group.append(pdf_radio)
        
        text_radio = Gtk.RadioButton.new_with_label_from_widget(all_radio, "Text")
        text_radio.connect("toggled", lambda x: self.update_pattern_display() if x.get_active() else None)
        text_radio.file_pattern = "*.{txt,md,org,rst,log}"
        self.filetype_group.append(text_radio)
        
        # First row
        filetype_button_box1.pack_start(all_radio, False, False, 0)
        filetype_button_box1.pack_start(images_radio, False, False, 0)
        filetype_button_box1.pack_start(pdf_radio, False, False, 0)
        filetype_button_box1.pack_start(text_radio, False, False, 0)
        
        video_radio = Gtk.RadioButton.new_with_label_from_widget(all_radio, "Videos")
        video_radio.connect("toggled", lambda x: self.update_pattern_display() if x.get_active() else None)
        video_radio.file_pattern = "*.{mp4,avi,mkv,mov,wmv,flv,webm,m4v,mpg,mpeg}"
        self.filetype_group.append(video_radio)
        
        audio_radio = Gtk.RadioButton.new_with_label_from_widget(all_radio, "Audio")
        audio_radio.connect("toggled", lambda x: self.update_pattern_display() if x.get_active() else None)
        audio_radio.file_pattern = "*.{mp3,wav,flac,m4a,aac,ogg,wma,opus}"
        self.filetype_group.append(audio_radio)
        
        docs_radio = Gtk.RadioButton.new_with_label_from_widget(all_radio, "Docs")
        docs_radio.connect("toggled", lambda x: self.update_pattern_display() if x.get_active() else None)
        docs_radio.file_pattern = "*.{doc,docx,odt,pdf,rtf}"
        self.filetype_group.append(docs_radio)
        
        # Second row
        filetype_button_box2.pack_start(video_radio, False, False, 0)
        filetype_button_box2.pack_start(audio_radio, False, False, 0)
        filetype_button_box2.pack_start(docs_radio, False, False, 0)
        
        builder_box.pack_start(filetype_button_box1, False, False, 0)
        builder_box.pack_start(filetype_button_box2, False, False, 0)
        
        # Contains text
        contains_label = Gtk.Label()
        contains_label.set_markup("<b>Contains:</b>")
        contains_label.set_xalign(0)
        builder_box.pack_start(contains_label, False, False, 2)
        
        self.contains_entry = Gtk.Entry()
        self.contains_entry.set_placeholder_text("vacation, report...")
        self.contains_entry.set_width_chars(20)
        self.contains_entry.connect("changed", lambda x: self.update_pattern_display())
        builder_box.pack_start(self.contains_entry, False, False, 0)
        
        # Pattern display
        pattern_display_label = Gtk.Label()
        pattern_display_label.set_markup("<b>Pattern:</b>")
        pattern_display_label.set_xalign(0)
        builder_box.pack_start(pattern_display_label, False, False, 2)
        
        self.pattern_display = Gtk.Entry()
        self.pattern_display.set_editable(True)
        self.pattern_display.set_text("*")
        self.pattern_display.set_width_chars(25)
        builder_box.pack_start(self.pattern_display, False, False, 0)
        
        builder_frame.add(builder_box)
        top_hbox.pack_start(builder_frame, True, True, 0)  # Expand to fill
        
        # COLUMN 2: Network Settings
        settings_frame = Gtk.Frame(label="Network Settings")
        settings_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=3)
        settings_box.set_border_width(5)
        
        # Network input
        network_label = Gtk.Label()
        network_label.set_markup("<b>Range:</b>")
        network_label.set_xalign(0)
        settings_box.pack_start(network_label, False, False, 0)
        
        self.network_entry = Gtk.Entry()
        self.network_entry.set_placeholder_text("auto or 192.168.1.0/24")
        settings_box.pack_start(self.network_entry, False, False, 0)
        
        # SMB credentials
        smb_label = Gtk.Label()
        smb_label.set_markup("<b>SMB (optional):</b>")
        smb_label.set_xalign(0)
        settings_box.pack_start(smb_label, False, False, 2)
        
        self.username_entry = Gtk.Entry()
        self.username_entry.set_placeholder_text("Username")
        settings_box.pack_start(self.username_entry, False, False, 0)
        
        self.password_entry = Gtk.Entry()
        self.password_entry.set_placeholder_text("Password")
        self.password_entry.set_visibility(False)
        self.password_entry.set_invisible_char('•')
        settings_box.pack_start(self.password_entry, False, False, 0)
        
        settings_frame.add(settings_box)
        top_hbox.pack_start(settings_frame, True, True, 0)  # Expand to fill
        
        # COLUMN 3: Action Buttons and Protocols
        right_column_frame = Gtk.Frame(label="Actions")
        right_column_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=5)
        right_column_box.set_border_width(5)
        
        # Protocol checkboxes (horizontal)
        protocol_label = Gtk.Label()
        protocol_label.set_markup("<b>Search:</b>")
        protocol_label.set_xalign(0)
        right_column_box.pack_start(protocol_label, False, False, 0)
        
        protocol_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        
        self.local_check = Gtk.CheckButton(label="Local")
        self.local_check.set_active(True)
        protocol_box.pack_start(self.local_check, False, False, 0)
        
        self.smb_check = Gtk.CheckButton(label="SMB")
        self.smb_check.set_active(True)
        protocol_box.pack_start(self.smb_check, False, False, 0)
        
        self.nfs_check = Gtk.CheckButton(label="NFS")
        self.nfs_check.set_active(True)
        protocol_box.pack_start(self.nfs_check, False, False, 0)
        
        right_column_box.pack_start(protocol_box, False, False, 0)
        
        # Separator
        separator = Gtk.Separator(orientation=Gtk.Orientation.HORIZONTAL)
        right_column_box.pack_start(separator, False, False, 3)
        
        # Search button (normal size)
        self.search_button = Gtk.Button(label="🔍 Search")
        self.search_button.connect("clicked", self.on_search_clicked)
        self.search_button.set_size_request(120, 35)
        right_column_box.pack_start(self.search_button, False, False, 0)
        
        # Stop button (normal size)
        self.stop_button = Gtk.Button(label="⏹ Stop")
        self.stop_button.connect("clicked", self.on_stop_clicked)
        self.stop_button.set_sensitive(False)
        self.stop_button.set_size_request(120, 35)
        right_column_box.pack_start(self.stop_button, False, False, 0)
        
        # Clear button (smaller)
        self.clear_button = Gtk.Button(label="Clear Results")
        self.clear_button.connect("clicked", self.on_clear_clicked)
        self.clear_button.set_size_request(120, 30)
        right_column_box.pack_start(self.clear_button, False, False, 0)
        
        right_column_frame.add(right_column_box)
        top_hbox.pack_start(right_column_frame, True, True, 0)  # Expand to fill
        
        vbox.pack_start(top_hbox, False, False, 0)
        
        # Progress/status section
        status_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        self.status_label = Gtk.Label(label="Ready to search")
        self.status_label.set_xalign(0)
        self.progress_bar = Gtk.ProgressBar()
        self.progress_bar.set_show_text(True)
        status_box.pack_start(self.status_label, False, False, 0)
        status_box.pack_start(self.progress_bar, True, True, 0)
        vbox.pack_start(status_box, False, False, 0)
        
        # Results section
        results_frame = Gtk.Frame(label="Search Results")
        results_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=5)
        results_box.set_border_width(5)
        
        # Create scrolled window for results
        scrolled = Gtk.ScrolledWindow()
        scrolled.set_hexpand(True)
        scrolled.set_vexpand(True)
        
        # TreeView for results
        self.results_store = Gtk.ListStore(str, str, str)  # Path, Modified Date, File Size
        self.results_view = Gtk.TreeView(model=self.results_store)
        self.results_view.set_enable_search(True)
        self.results_view.set_search_column(0)
        
        # Add double-click handler
        self.results_view.connect("row-activated", self.on_result_double_click)
        
        # Path column
        renderer = Gtk.CellRendererText()
        renderer.set_property("font", "monospace")
        renderer.set_property("ellipsize", Pango.EllipsizeMode.MIDDLE)
        column = Gtk.TreeViewColumn("Network Path", renderer, text=0)
        column.set_resizable(True)
        column.set_expand(True)
        column.set_min_width(400)
        self.results_view.append_column(column)
        
        # Last Modified column
        renderer = Gtk.CellRendererText()
        column = Gtk.TreeViewColumn("Last Modified", renderer, text=1)
        column.set_resizable(True)
        column.set_min_width(150)
        self.results_view.append_column(column)
        
        # File Size column
        renderer = Gtk.CellRendererText()
        renderer.set_property("xalign", 1.0)  # Right-align numbers
        column = Gtk.TreeViewColumn("Size", renderer, text=2)
        column.set_resizable(True)
        column.set_min_width(100)
        self.results_view.append_column(column)
        
        # Enhanced context menu for right-click
        self.results_view.connect("button-press-event", self.on_results_click)
        
        scrolled.add(self.results_view)
        results_box.pack_start(scrolled, True, True, 0)
        
        # Results count and sort controls
        results_controls_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        
        self.results_count_label = Gtk.Label(label="0 files found")
        self.results_count_label.set_xalign(0)
        results_controls_box.pack_start(self.results_count_label, False, False, 0)
        
        # Sort button
        sort_label = Gtk.Label(label="Sort by:")
        results_controls_box.pack_start(sort_label, False, False, 0)
        
        self.sort_location_button = Gtk.RadioButton(label="Location")
        self.sort_location_button.set_active(True)
        self.sort_location_button.connect("toggled", self.on_sort_changed)
        results_controls_box.pack_start(self.sort_location_button, False, False, 0)
        
        self.sort_filename_button = Gtk.RadioButton.new_with_label_from_widget(
            self.sort_location_button, "Filename"
        )
        self.sort_filename_button.connect("toggled", self.on_sort_changed)
        results_controls_box.pack_start(self.sort_filename_button, False, False, 0)
        
        results_box.pack_start(results_controls_box, False, False, 0)
        
        results_frame.add(results_box)
        vbox.pack_start(results_frame, True, True, 0)
        
        # Log text view
        log_frame = Gtk.Frame(label="Search Log")
        log_scroll = Gtk.ScrolledWindow()
        log_scroll.set_size_request(-1, 80)  # Reduced from 100 to 80
        
        self.log_buffer = Gtk.TextBuffer()
        self.log_view = Gtk.TextView(buffer=self.log_buffer)
        self.log_view.set_editable(False)
        self.log_view.set_wrap_mode(Gtk.WrapMode.WORD)
        self.log_view.set_cursor_visible(False)
        
        # Monospace font for log
        font_desc = Pango.FontDescription("monospace 9")
        self.log_view.override_font(font_desc)
        
        log_scroll.add(self.log_view)
        log_frame.add(log_scroll)
        vbox.pack_start(log_frame, False, False, 0)
        
        self.connect("destroy", self.on_destroy)
        
    def log(self, message, level="INFO"):
        """Add message to log view"""
        def _log():
            end_iter = self.log_buffer.get_end_iter()
            timestamp = GLib.DateTime.new_now_local().format("%H:%M:%S")
            self.log_buffer.insert(end_iter, f"[{timestamp}] [{level}] {message}\n")
            
            # Auto-scroll to bottom
            mark = self.log_buffer.create_mark(None, end_iter, False)
            self.log_view.scroll_to_mark(mark, 0.0, True, 0.0, 1.0)
            
        GLib.idle_add(_log)
    
    def update_pattern_display(self):
        """Update the pattern display based on selected file type and contains text"""
        # Get selected file type pattern
        file_pattern = "*"
        for radio in self.filetype_group:
            if radio.get_active():
                file_pattern = radio.file_pattern
                break
        
        # Get contains text
        contains_text = self.contains_entry.get_text().strip()
        
        # Build combined pattern
        if contains_text:
            if file_pattern == "*":
                # Just contains, no specific type
                pattern = f"*{contains_text}*"
            else:
                # Combine contains with file type
                # For patterns like *.pdf, make it *contains*.pdf
                # For patterns like *.{ext1,ext2}, make it *contains*.{ext1,ext2}
                if file_pattern.startswith("*."):
                    # Add * after contains text to match anything between contains and extension
                    pattern = f"*{contains_text}*{file_pattern[1:]}"
                else:
                    pattern = f"*{contains_text}*"
        else:
            # No contains text, just use file type pattern
            pattern = file_pattern
        
        self.pattern_display.set_text(pattern)
    
    def search_local_drives(self, pattern):
        """Search all mounted local drives"""
        try:
            # Get list of mounted filesystems
            result = subprocess.run(
                ['df', '-h', '--output=target'],
                capture_output=True,
                text=True,
                timeout=10  # Increased from 5 to 10
            )
            
            if result.returncode != 0:
                self.log("Could not list local drives", "ERROR")
                return
            
            # Parse mount points
            mount_points = []
            for line in result.stdout.split('\n')[1:]:  # Skip header
                mount = line.strip()
                if mount and mount != '/':
                    # Filter out special filesystems
                    if not any(mount.startswith(x) for x in ['/proc', '/sys', '/dev', '/run', '/tmp', '/snap', '/boot']):
                        mount_points.append(mount)
            
            # Always search root
            mount_points.insert(0, '/')
            
            self.log(f"Found {len(mount_points)} local mount point(s) to search")
            
            # Convert pattern to find-compatible format
            find_pattern = self.convert_pattern_for_find(pattern)
            
            # Build exclusion paths for system directories and hidden folders
            exclusions = self.build_exclusion_paths()
            
            # Search each mount point
            for idx, mount_point in enumerate(mount_points):
                if not self.search_running:
                    break
                
                self.log(f"Searching local: {mount_point} (excluding system & hidden folders)")
                
                try:
                    # Use find to search this mount point with exclusions
                    # Format: find /path \( exclusion-paths \) -prune -o -type f pattern -print
                    find_cmd = f'find "{mount_point}" {exclusions} -prune -o -type f {find_pattern} -print 2>/dev/null'
                    
                    self.log(f"Running: {find_cmd[:150]}...")  # Log abbreviated command
                    
                    find_result = subprocess.run(
                        find_cmd,
                        shell=True,
                        capture_output=True,
                        text=True,
                        timeout=300  # Increased to 5 minutes per drive
                    )
                    
                    # Add results
                    count = 0
                    for line in find_result.stdout.strip().split('\n'):
                        if line and self.search_running:
                            # Get file info
                            mod_time, file_size = self.get_file_info(line)
                            # Format as file:// URI
                            self.add_result(f"file://{line}", mod_time, file_size)
                            count += 1
                    
                    if count > 0:
                        self.log(f"Found {count} file(s) in {mount_point}")
                    else:
                        self.log(f"No matches in {mount_point}")
                            
                except subprocess.TimeoutExpired:
                    self.log(f"Timeout searching {mount_point} (large drive, search continues...)", "WARN")
                except Exception as e:
                    self.log(f"Error searching {mount_point}: {e}", "ERROR")
            
            self.log("Local drive search complete")
            
        except subprocess.TimeoutExpired:
            self.log("Timeout listing drives", "ERROR")
        except Exception as e:
            self.log(f"Error searching local drives: {e}", "ERROR")
    
    def build_exclusion_paths(self):
        """Build find command exclusion patterns for system folders and hidden directories"""
        # System directories to exclude
        system_dirs = [
            '/proc', '/sys', '/dev', '/run', '/tmp', '/var/tmp', 
            '/snap', '/boot', '/lost+found', '/var/cache', '/var/log', '/var/spool'
        ]
        
        # Build exclusion expression
        # \( -path '/proc' -o -path '/sys' ... -o -path '*/.*' \)
        exclusion_parts = [f"-path '{d}'" for d in system_dirs]
        # Add hidden directories exclusion (any directory starting with .)
        exclusion_parts.append("-path '*/.*'")
        
        return '\\( ' + ' -o '.join(exclusion_parts) + ' \\)'
    
    def convert_pattern_for_find(self, pattern):
        """Convert shell pattern to find command pattern"""
        # Handle brace expansion like *.{txt,md,org}
        if '{' in pattern and '}' in pattern:
            # Extract the extensions
            match = re.search(r'\*\.{([^}]+)}', pattern)
            if match:
                extensions = match.group(1).split(',')
                # Build find expression with -o (OR)
                conditions = []
                for ext in extensions:
                    conditions.append(f'-iname "{pattern.replace("{" + match.group(1) + "}", ext.strip())}"')
                return '\\( ' + ' -o '.join(conditions) + ' \\)'
        
        # Simple pattern, just use -iname
        return f'-iname "{pattern}"'
    
    def format_file_size(self, size_bytes):
        """Format file size in human-readable format"""
        try:
            size = int(size_bytes)
            for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
                if size < 1024.0:
                    return f"{size:.1f} {unit}"
                size /= 1024.0
            return f"{size:.1f} PB"
        except:
            return "Unknown"
    
    def get_file_info(self, filepath):
        """Get file modification time and size"""
        try:
            stat_info = os.stat(filepath)
            mod_time = datetime.datetime.fromtimestamp(stat_info.st_mtime).strftime('%Y-%m-%d %H:%M')
            file_size = self.format_file_size(stat_info.st_size)
            return mod_time, file_size
        except:
            return "Unknown", "Unknown"
    
    def update_status(self, message):
        """Update status label"""
        GLib.idle_add(self.status_label.set_text, message)
    
    def update_progress(self, fraction, text=None):
        """Update progress bar"""
        def _update():
            self.progress_bar.set_fraction(fraction)
            if text:
                self.progress_bar.set_text(text)
        GLib.idle_add(_update)
    
    def add_result(self, path, modified_date, file_size):
        """Add result to tree view"""
        def _add():
            self.results_store.append([path, modified_date, file_size])
            count = len(self.results_store)
            self.results_count_label.set_text(f"{count} file(s) found")
        GLib.idle_add(_add)
    
    def on_results_click(self, widget, event):
        """Handle right-click on results"""
        if event.type == getattr(gi.repository.Gdk.EventType, 'BUTTON_PRESS') and event.button == 3:
            selection = self.results_view.get_selection()
            model, tree_iter = selection.get_selected()
            if tree_iter:
                path = model[tree_iter][0]
                
                menu = Gtk.Menu()
                
                # Open file
                open_item = Gtk.MenuItem(label="Open File")
                open_item.connect("activate", lambda x: self.open_network_file(path))
                menu.append(open_item)
                
                # Open in file manager
                file_mgr_item = Gtk.MenuItem(label="Open in File Manager")
                file_mgr_item.connect("activate", lambda x: self.open_in_file_manager(path))
                menu.append(file_mgr_item)
                
                # Open with...
                open_with_item = Gtk.MenuItem(label="Open With...")
                open_with_item.connect("activate", lambda x: self.open_with_dialog(path))
                menu.append(open_with_item)
                
                menu.append(Gtk.SeparatorMenuItem())
                
                # Copy path
                copy_item = Gtk.MenuItem(label="Copy Path")
                copy_item.connect("activate", lambda x: self.copy_to_clipboard(path))
                menu.append(copy_item)
                
                menu.show_all()
                menu.popup(None, None, None, None, event.button, event.time)
    
    def on_result_double_click(self, tree_view, path, column):
        """Handle double-click on result - open the file"""
        model = tree_view.get_model()
        tree_iter = model.get_iter(path)
        if tree_iter:
            network_path = model[tree_iter][0]
            self.open_network_file(network_path)
    
    def get_original_user(self):
        """Get the original user who launched the app (before sudo/pkexec)"""
        # Try SUDO_USER first (if launched via sudo)
        user = os.environ.get('SUDO_USER')
        if user:
            return user
        
        # Try PKEXEC_UID (if launched via pkexec)
        uid = os.environ.get('PKEXEC_UID')
        if uid:
            try:
                return pwd.getpwuid(int(uid)).pw_name
            except:
                pass
        
        # Fallback to current user
        try:
            return pwd.getpwuid(os.getuid()).pw_name
        except:
            return None
    
    def run_as_user(self, command):
        """Run a command as the original user, not as root"""
        user = self.get_original_user()
        
        if user and os.geteuid() == 0:  # If we're running as root
            # Run command as original user with their environment
            display = os.environ.get('DISPLAY', ':0')
            xauthority = os.environ.get('XAUTHORITY', f'/home/{user}/.Xauthority')
            dbus = os.environ.get('DBUS_SESSION_BUS_ADDRESS', '')
            
            # Build command to run as user with proper environment
            if isinstance(command, list):
                cmd_str = ' '.join([f"'{c}'" if ' ' in c else c for c in command])
            else:
                cmd_str = command
            
            full_cmd = [
                'sudo', '-u', user,
                'DISPLAY=' + display,
                'XAUTHORITY=' + xauthority
            ]
            
            if dbus:
                full_cmd.append('DBUS_SESSION_BUS_ADDRESS=' + dbus)
            
            if isinstance(command, list):
                full_cmd.extend(command)
            else:
                full_cmd.extend(['sh', '-c', cmd_str])
            
            return subprocess.Popen(full_cmd)
        else:
            # Not running as root, just run normally
            if isinstance(command, list):
                return subprocess.Popen(command)
            else:
                return subprocess.Popen(command, shell=True)
    
    def network_path_to_uri(self, network_path):
        """Convert network path to URI"""
        # Already a URI
        if network_path.startswith('file://') or network_path.startswith('smb://') or network_path.startswith('nfs://'):
            return network_path
        # Convert //host/share/path to smb://host/share/path
        elif network_path.startswith('//'):
            return 'smb:' + network_path
        # Handle NFS paths like host:/export/path
        elif ':' in network_path and not network_path.startswith('smb://'):
            # NFS path - convert to nfs:// URI
            parts = network_path.split(':', 1)
            return f'nfs://{parts[0]}{parts[1]}'
        return network_path
    
    def open_network_file(self, network_path):
        """Open network file with default application"""
        try:
            uri = self.network_path_to_uri(network_path)
            self.log(f"Opening: {network_path}")
            
            # Run xdg-open as the original user, not as root
            process = self.run_as_user(['xdg-open', uri])
            
            # Don't wait for the process - let it run in background
            self.log(f"Launched application for: {network_path}")
                
        except Exception as e:
            self.log(f"Error opening file: {e}", "ERROR")
            dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="Error Opening File",
            )
            dialog.format_secondary_text(f"Failed to open:\n{network_path}\n\nError: {str(e)}")
            dialog.run()
            dialog.destroy()
    
    def open_in_file_manager(self, network_path):
        """Open the directory containing the file in file manager"""
        try:
            # Handle local files
            if network_path.startswith('file://'):
                file_path = network_path[7:]  # Remove file://
                dir_path = os.path.dirname(file_path)
                uri = f'file://{dir_path}'
            # Get the directory path for network files
            elif network_path.startswith('//'):
                # SMB path: //host/share/path/to/file -> //host/share/path/to
                parts = network_path.rsplit('/', 1)
                dir_path = parts[0] if len(parts) > 1 else network_path
                uri = self.network_path_to_uri(dir_path)
            else:
                # NFS path: host:/export/path/to/file -> host:/export/path/to
                parts = network_path.rsplit('/', 1)
                dir_path = parts[0] if len(parts) > 1 else network_path
                uri = self.network_path_to_uri(dir_path)
            
            self.log(f"Opening directory: {uri}")
            
            # Run xdg-open as the original user, not as root
            process = self.run_as_user(['xdg-open', uri])
            self.log(f"Opened directory successfully")
                
        except Exception as e:
            self.log(f"Error opening directory: {e}", "ERROR")
    
    def open_with_dialog(self, network_path):
        """Show dialog to select application to open file with"""
        try:
            # Get the file URI
            uri = self.network_path_to_uri(network_path)
            
            # Create a GFile from the URI to get mime type
            gfile = gi.repository.Gio.File.new_for_uri(uri)
            
            # Create AppChooserDialog
            dialog = Gtk.AppChooserDialog.new_for_content_type(
                self,
                Gtk.DialogFlags.MODAL,
                gfile.query_info('standard::content-type', gi.repository.Gio.FileQueryInfoFlags.NONE, None).get_content_type()
            )
            dialog.set_heading("Choose Application")
            
            response = dialog.run()
            
            if response == Gtk.ResponseType.OK:
                app_info = dialog.get_app_info()
                if app_info:
                    try:
                        self.log(f"Opening with {app_info.get_name()}: {network_path}")
                        
                        # Get the executable command
                        commandline = app_info.get_commandline()
                        if commandline:
                            # Replace %u, %U, %f, %F with the URI
                            commandline = commandline.replace('%u', uri).replace('%U', uri)
                            commandline = commandline.replace('%f', uri).replace('%F', uri)
                            # Remove any remaining % codes
                            commandline = re.sub(r'%[a-zA-Z]', '', commandline)
                            
                            # Run as original user
                            self.run_as_user(commandline)
                            self.log(f"Launched {app_info.get_name()}")
                        else:
                            # Fallback: try to launch with app name
                            self.run_as_user([app_info.get_executable(), uri])
                            self.log(f"Launched {app_info.get_name()}")
                            
                    except Exception as e:
                        self.log(f"Error launching application: {e}", "ERROR")
            
            dialog.destroy()
            
        except Exception as e:
            self.log(f"Error showing app chooser: {e}", "ERROR")
            # Fallback to simple text entry if AppChooser fails
            self.open_with_dialog_fallback(network_path)
    
    def open_with_dialog_fallback(self, network_path):
        """Fallback dialog with text entry if AppChooser fails"""
        dialog = Gtk.Dialog(
            title="Open With...",
            transient_for=self,
            flags=0
        )
        dialog.add_buttons(
            Gtk.STOCK_CANCEL, Gtk.ResponseType.CANCEL,
            Gtk.STOCK_OK, Gtk.ResponseType.OK
        )
        
        content_area = dialog.get_content_area()
        content_area.set_spacing(10)
        content_area.set_border_width(10)
        
        label = Gtk.Label(label="Enter command to open file with:")
        content_area.pack_start(label, False, False, 0)
        
        entry = Gtk.Entry()
        entry.set_placeholder_text("e.g., vlc, gimp, libreoffice")
        entry.set_width_chars(40)
        content_area.pack_start(entry, False, False, 0)
        
        help_label = Gtk.Label()
        help_label.set_markup('<span size="small">Common apps: vlc, mpv, gimp, eog, firefox, libreoffice</span>')
        content_area.pack_start(help_label, False, False, 0)
        
        dialog.show_all()
        
        response = dialog.run()
        
        if response == Gtk.ResponseType.OK:
            command = entry.get_text().strip()
            if command:
                uri = self.network_path_to_uri(network_path)
                try:
                    self.log(f"Opening with {command}: {network_path}")
                    # Run as original user
                    self.run_as_user([command, uri])
                    self.log(f"Launched {command}")
                except Exception as e:
                    self.log(f"Error launching {command}: {e}", "ERROR")
        
        dialog.destroy()
    
    def copy_to_clipboard(self, text):
        """Copy text to clipboard"""
        clipboard = Gtk.Clipboard.get(gi.repository.Gdk.SELECTION_CLIPBOARD)
        clipboard.set_text(text, -1)
        self.log(f"Copied to clipboard: {text}")
    
    def on_clear_clicked(self, button):
        """Clear results and log"""
        self.results_store.clear()
        self.log_buffer.set_text("")
        self.results_count_label.set_text("0 files found")
        self.log("Results cleared")
    
    def on_sort_changed(self, button):
        """Handle sort option change"""
        if not button.get_active():
            return
        
        # Get all current results
        results = []
        for row in self.results_store:
            results.append((row[0], row[1], row[2]))  # path, modified_date, file_size
        
        if len(results) == 0:
            return
        
        # Sort based on selection
        if self.sort_filename_button.get_active():
            # Sort by filename (basename of path)
            results.sort(key=lambda x: os.path.basename(x[0]).lower())
            self.log("Sorted by filename")
        else:
            # Sort by location (full path)
            results.sort(key=lambda x: x[0].lower())
            self.log("Sorted by location")
        
        # Clear and repopulate
        self.results_store.clear()
        for path, modified_date, file_size in results:
            self.results_store.append([path, modified_date, file_size])
    
    def on_stop_clicked(self, button):
        """Stop search"""
        self.search_running = False
        self.log("Search stopped by user", "WARN")
        # Re-enable search button
        self.search_button.set_sensitive(True)
        self.stop_button.set_sensitive(False)
        self.clear_button.set_sensitive(True)
    
    def on_search_clicked(self, button):
        """Start search in background thread"""
        # Validate inputs
        pattern = self.pattern_display.get_text().strip()
        if not pattern:
            dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="No Search Pattern",
            )
            dialog.format_secondary_text("Please select a file type or enter text to search for.")
            dialog.run()
            dialog.destroy()
            return
        
        network = self.network_entry.get_text().strip() or None
        username = self.username_entry.get_text().strip() or None
        password = self.password_entry.get_text().strip() or None
        search_local = self.local_check.get_active()
        search_smb = self.smb_check.get_active()
        search_nfs = self.nfs_check.get_active()
        
        if not search_local and not search_smb and not search_nfs:
            dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="No Search Location Selected",
            )
            dialog.format_secondary_text("Please select at least one location to search (Local, SMB, or NFS).")
            dialog.run()
            dialog.destroy()
            return
        
        # Check if running as root
        if subprocess.run(['id', '-u'], capture_output=True, text=True).stdout.strip() != '0':
            dialog = Gtk.MessageDialog(
                transient_for=self,
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="Root Privileges Required",
            )
            dialog.format_secondary_text(
                "This application requires root privileges to mount network shares.\n\n"
                "Please run with: sudo ./network_file_search_gui.py"
            )
            dialog.run()
            dialog.destroy()
            return
        
        # Start search
        self.search_running = True
        self.search_button.set_sensitive(False)
        self.stop_button.set_sensitive(True)
        self.clear_button.set_sensitive(False)
        
        self.search_thread = threading.Thread(
            target=self.run_search,
            args=(pattern, network, username, password, search_local, search_smb, search_nfs)
        )
        self.search_thread.daemon = True
        self.search_thread.start()
    
    def run_search(self, pattern, network, username, password, search_local, search_smb, search_nfs):
        """Run search in background thread"""
        try:
            self.log(f"Starting search for: {pattern}")
            self.update_progress(0.0, "Initializing...")
            
            # Search local drives first
            if search_local and self.search_running:
                self.update_progress(0.05, "Searching local drives...")
                self.log("Searching local drives...")
                self.search_local_drives(pattern)
            
            # Then search network if requested
            if (search_smb or search_nfs) and self.search_running:
                # Auto-detect network
                if not network:
                    network = self.get_local_network()
                    if network:
                        self.log(f"Auto-detected network: {network}")
                    else:
                        self.log("Could not auto-detect network", "ERROR")
                        if not search_local:  # Only error if we didn't search local
                            self.update_status("Error: Could not detect network")
                        return
                
                self.update_status(f"Scanning network {network}...")
            
            # Search SMB
            if search_smb and self.search_running:
                self.update_progress(0.1, "Finding SMB hosts...")
                smb_hosts = self.find_smb_hosts(network)
                
                if smb_hosts:
                    total_hosts = len(smb_hosts)
                    for idx, host in enumerate(smb_hosts):
                        if not self.search_running:
                            break
                        
                        progress = 0.1 + (0.4 * (idx / total_hosts))
                        self.update_progress(progress, f"Searching {host}...")
                        self.log(f"Checking SMB host: {host}")
                        
                        shares = self.get_smb_shares(host, username, password)
                        for share in shares:
                            if not self.search_running:
                                break
                            
                            self.log(f"  Searching share: {share} (may take up to 5 min for large drives)")
                            results = self.search_smb_share(host, share, pattern, username, password)
                            for result in results:
                                # result is now a tuple (path, mod_time, file_size)
                                self.add_result(result[0], result[1], result[2])
            
            # Search NFS
            if search_nfs and self.search_running:
                self.update_progress(0.5, "Checking for NFS exports...")
                network_obj = ipaddress.ip_network(network, strict=False)
                hosts = list(network_obj.hosts())[:20]  # Limit to 20
                
                total_hosts = len(hosts)
                for idx, ip in enumerate(hosts):
                    if not self.search_running:
                        break
                    
                    ip_str = str(ip)
                    progress = 0.5 + (0.4 * (idx / total_hosts))
                    self.update_progress(progress, f"Checking NFS on {ip_str}...")
                    
                    results = self.search_nfs_exports(ip_str, pattern)
                    for result in results:
                        # result is now a tuple (path, mod_time, file_size)
                        self.add_result(result[0], result[1], result[2])
            
            if self.search_running:
                self.update_progress(1.0, "Search complete")
                self.update_status("Search complete")
                self.log("Search finished successfully")
            else:
                self.update_progress(0.0, "Search stopped")
                self.update_status("Search stopped by user")
                
        except Exception as e:
            self.log(f"Error during search: {e}", "ERROR")
            self.update_status(f"Error: {e}")
        finally:
            GLib.idle_add(self.search_button.set_sensitive, True)
            GLib.idle_add(self.stop_button.set_sensitive, False)
            GLib.idle_add(self.clear_button.set_sensitive, True)
            self.cleanup()
    
    def get_local_network(self):
        """Get local network CIDR"""
        try:
            result = subprocess.run(['ip', 'route', 'show', 'default'], 
                                  capture_output=True, text=True, timeout=5)
            match = re.search(r'dev\s+(\S+)', result.stdout)
            if not match:
                return None
            
            interface = match.group(1)
            result = subprocess.run(['ip', 'addr', 'show', interface],
                                  capture_output=True, text=True, timeout=5)
            match = re.search(r'inet\s+(\d+\.\d+\.\d+\.\d+/\d+)', result.stdout)
            if match:
                return match.group(1)
        except Exception as e:
            self.log(f"Error detecting network: {e}", "ERROR")
        return None
    
    def find_smb_hosts(self, network_cidr):
        """Find SMB hosts on network"""
        smb_hosts = []
        try:
            if shutil.which('nmap'):
                result = subprocess.run(
                    ['nmap', '-p', '445', '--open', '-oG', '-', network_cidr],
                    capture_output=True, text=True, timeout=120
                )
                for line in result.stdout.split('\n'):
                    if 'Host:' in line and 'Status: Up' in line:
                        match = re.search(r'Host:\s+(\d+\.\d+\.\d+\.\d+)', line)
                        if match:
                            smb_hosts.append(match.group(1))
        except Exception as e:
            self.log(f"Error finding SMB hosts: {e}", "ERROR")
        
        return smb_hosts
    
    def get_smb_shares(self, host, username=None, password=None):
        """List SMB shares"""
        shares = []
        try:
            cmd = ['smbclient', '-L', f'//{host}', '-N']
            stdin_input = None
            
            if username:
                cmd = ['smbclient', '-L', f'//{host}', '-U', username]
                if password:
                    stdin_input = password + '\n'
                else:
                    stdin_input = '\n'
            
            result = subprocess.run(cmd, capture_output=True, text=True, 
                                  timeout=10, input=stdin_input)
            
            in_shares = False
            for line in result.stdout.split('\n'):
                if 'Sharename' in line:
                    in_shares = True
                    continue
                if in_shares and line.strip():
                    if 'IPC$' in line or 'print$' in line.lower():
                        continue
                    parts = line.split()
                    if parts and not parts[0].startswith('-'):
                        shares.append(parts[0])
        except Exception as e:
            self.log(f"Error listing shares on {host}: {e}", "ERROR")
        
        return shares
    
    def search_smb_share(self, host, share, pattern, username=None, password=None):
        """Search SMB share"""
        results = []
        mount_point = None
        
        try:
            mount_point = tempfile.mkdtemp(prefix='smb_search_')
            self.temp_dirs.append(mount_point)
            
            # Build mount options
            if username and password:
                mount_opts = f'username={username},password={password},ro,vers=3.0'
            elif username:
                mount_opts = f'username={username},ro,vers=3.0'
            else:
                mount_opts = 'guest,ro,vers=3.0'
            
            result = subprocess.run([
                'sudo', 'mount', '-t', 'cifs',
                f'//{host}/{share}',
                mount_point,
                '-o', mount_opts
            ], capture_output=True, text=True, timeout=30)  # Increased from 10 to 30
            
            if result.returncode == 0:
                # Convert pattern for find
                find_pattern = self.convert_pattern_for_find(pattern)
                
                # Exclude hidden directories from network shares too
                exclusion = "\\( -path '*/.*' \\)"
                
                find_result = subprocess.run(
                    f'find {mount_point} {exclusion} -prune -o -type f {find_pattern} -print 2>/dev/null',
                    shell=True, capture_output=True, text=True, timeout=300  # Increased from 60 to 300
                )
                
                for line in find_result.stdout.strip().split('\n'):
                    if line and line != mount_point:
                        # Get file info from mounted path
                        mod_time, file_size = self.get_file_info(line)
                        # Convert to network path
                        rel_path = line.replace(mount_point, '').lstrip('/')
                        results.append((f"//{host}/{share}/{rel_path}", mod_time, file_size))
                
                subprocess.run(['sudo', 'umount', mount_point],
                             stderr=subprocess.DEVNULL, timeout=5)
        except Exception as e:
            self.log(f"Error searching {host}/{share}: {e}", "ERROR")
        
        return results
    
    def search_nfs_exports(self, host, pattern):
        """Search NFS exports"""
        results = []
        try:
            result = subprocess.run(
                ['showmount', '-e', host],
                capture_output=True, text=True, timeout=10
            )
            
            if result.returncode == 0:
                exports = []
                for line in result.stdout.split('\n')[1:]:
                    if line.strip():
                        export = line.split()[0]
                        exports.append(export)
                
                for export in exports:
                    mount_point = tempfile.mkdtemp(prefix='nfs_search_')
                    self.temp_dirs.append(mount_point)
                    
                    result = subprocess.run([
                        'sudo', 'mount', '-t', 'nfs',
                        f'{host}:{export}',
                        mount_point,
                        '-o', 'ro'
                    ], capture_output=True, text=True, timeout=30)  # Increased from 10 to 30
                    
                    if result.returncode == 0:
                        # Convert pattern for find
                        find_pattern = self.convert_pattern_for_find(pattern)
                        
                        # Exclude hidden directories from network shares too
                        exclusion = "\\( -path '*/.*' \\)"
                        
                        find_result = subprocess.run(
                            f'find {mount_point} {exclusion} -prune -o -type f {find_pattern} -print 2>/dev/null',
                            shell=True, capture_output=True, text=True, timeout=300  # Increased from 60 to 300
                        )
                        
                        for line in find_result.stdout.strip().split('\n'):
                            if line and line != mount_point:
                                # Get file info from mounted path
                                mod_time, file_size = self.get_file_info(line)
                                # Convert to network path
                                rel_path = line.replace(mount_point, '').lstrip('/')
                                results.append((f"{host}:{export}/{rel_path}", mod_time, file_size))
                        
                        subprocess.run(['sudo', 'umount', mount_point],
                                     stderr=subprocess.DEVNULL, timeout=5)
        except Exception as e:
            self.log(f"Error checking NFS on {host}: {e}", "ERROR")
        
        return results
    
    def cleanup(self):
        """Clean up temp directories"""
        for mount_point in self.temp_dirs:
            try:
                subprocess.run(['sudo', 'umount', mount_point],
                             stderr=subprocess.DEVNULL, check=False)
                shutil.rmtree(mount_point, ignore_errors=True)
            except:
                pass
        self.temp_dirs = []
    
    def on_destroy(self, widget):
        """Clean up on exit"""
        self.search_running = False
        if self.search_thread and self.search_thread.is_alive():
            self.search_thread.join(timeout=2)
        self.cleanup()
        Gtk.main_quit()

def main():
    import sys
    import os
    
    # Check if running as root
    if subprocess.run(['id', '-u'], capture_output=True, text=True).stdout.strip() != '0':
        # Not running as root - try to relaunch with pkexec
        print("Not running as root. Attempting to elevate privileges...")
        
        # Get the full path to this script
        script_path = os.path.abspath(__file__)
        
        # Get current display and xauthority for X11
        display = os.environ.get('DISPLAY', ':0')
        xauthority = os.environ.get('XAUTHORITY', os.path.expanduser('~/.Xauthority'))
        
        # Try pkexec first (modern standard)
        if shutil.which('pkexec'):
            try:
                # Use env to pass DISPLAY and XAUTHORITY to the elevated process
                env_cmd = ['env', f'DISPLAY={display}', f'XAUTHORITY={xauthority}']
                result = subprocess.run(
                    ['pkexec'] + env_cmd + [sys.executable, script_path] + sys.argv[1:],
                    check=False
                )
                sys.exit(result.returncode)
            except Exception as e:
                print(f"pkexec failed: {e}")
        
        # Try gksudo as fallback (older systems)
        elif shutil.which('gksudo'):
            try:
                result = subprocess.run(
                    ['gksudo', '--', sys.executable, script_path] + sys.argv[1:],
                    check=False
                )
                sys.exit(result.returncode)
            except Exception as e:
                print(f"gksudo failed: {e}")
        
        # Try kdesudo as fallback (KDE systems)
        elif shutil.which('kdesudo'):
            try:
                result = subprocess.run(
                    ['kdesudo', '--', sys.executable, script_path] + sys.argv[1:],
                    check=False
                )
                sys.exit(result.returncode)
            except Exception as e:
                print(f"kdesudo failed: {e}")
        
        # If we get here, no privilege escalation tool worked
        # Show error dialog
        try:
            Gtk.init_check()
            dialog = Gtk.MessageDialog(
                flags=0,
                message_type=Gtk.MessageType.ERROR,
                buttons=Gtk.ButtonsType.OK,
                text="Root Privileges Required",
            )
            dialog.format_secondary_text(
                "This application requires root privileges to mount network shares.\n\n"
                "No privilege escalation tool found (pkexec, gksudo, kdesudo).\n\n"
                "Please run from terminal with:\n"
                f"sudo {script_path}"
            )
            dialog.run()
            dialog.destroy()
        except:
            print("ERROR: This application requires root privileges.")
            print(f"Please run: sudo {script_path}")
        
        sys.exit(1)
    
    # Running as root - start the app
    app = NetworkFileSearchGUI()
    app.show_all()
    Gtk.main()

if __name__ == '__main__':
    import sys
    main()
