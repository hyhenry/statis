// Icon Picker Module - handles icon selection and upload

const IconPicker = (function() {
    // Private state
    let state = {
        sectionIndex: null,
        itemIndex: null,
        mode: null, // 'service' or 'favicon'
        icons: [],
        searchTimeout: null
    };

    // Callbacks for external integration
    let callbacks = {
        onServiceIconSelected: null,
        onFaviconSelected: null,
        renderServicesEditor: null
    };

    // Public API
    return {
        // Initialize with callbacks
        init: function(opts) {
            callbacks.onServiceIconSelected = opts.onServiceIconSelected;
            callbacks.onFaviconSelected = opts.onFaviconSelected;
            callbacks.renderServicesEditor = opts.renderServicesEditor;

            // Setup event listeners
            this._setupEventListeners();
        },

        // Open the icon picker
        open: function(sectionIndexOrMode, itemIndex) {
            // Check if opening for favicon (first arg is 'favicon' string)
            if (sectionIndexOrMode === 'favicon') {
                state.mode = 'favicon';
                state.sectionIndex = null;
                state.itemIndex = null;
            } else {
                state.mode = 'service';
                state.sectionIndex = sectionIndexOrMode;
                state.itemIndex = itemIndex;
            }

            const modal = document.getElementById('icon-picker-modal');
            modal.style.display = 'block';

            const searchInput = document.getElementById('icon-search');
            searchInput.value = '';
            searchInput.focus();

            // Load icons if not already loaded
            if (state.icons.length === 0) {
                this._loadIcons();
            } else {
                this._renderIconGrid(state.icons);
            }

            // Setup search with debounce
            const self = this;
            searchInput.oninput = function() {
                clearTimeout(state.searchTimeout);
                state.searchTimeout = setTimeout(() => {
                    self._searchIcons(this.value);
                }, 300);
            };

            // Initialize upload handlers
            setTimeout(() => this._initializeIconUpload(), 0);
        },

        // Close the icon picker
        close: function() {
            const modal = document.getElementById('icon-picker-modal');
            modal.style.display = 'none';
            state.sectionIndex = null;
            state.itemIndex = null;
            state.mode = null;
        },

        // Get current state (for external access)
        getState: function() {
            return { ...state };
        },

        // Private: Setup event listeners
        _setupEventListeners: function() {
            const self = this;

            // Close modal when clicking outside
            document.addEventListener('click', function(e) {
                const modal = document.getElementById('icon-picker-modal');
                if (e.target === modal) {
                    self.close();
                }
            });

            // Close modal on escape key
            document.addEventListener('keydown', function(e) {
                if (e.key === 'Escape') {
                    self.close();
                }
            });
        },

        // Private: Load icons from API
        _loadIcons: async function() {
            const grid = document.getElementById('icon-grid');
            const countEl = document.getElementById('icon-count');

            grid.innerHTML = '<div class="icon-picker-loading">Loading icons...</div>';

            try {
                const response = await fetch('/api/icons/search');
                const data = await response.json();

                state.icons = data.icons || [];
                countEl.textContent = `${state.icons.length} icons available`;
                this._renderIconGrid(state.icons);
            } catch (error) {
                console.error('Failed to load icons:', error);
                grid.innerHTML = '<div class="icon-picker-loading">Failed to load icons</div>';
                countEl.textContent = 'Error loading icons';
            }
        },

        // Private: Search icons
        _searchIcons: async function(query) {
            const grid = document.getElementById('icon-grid');
            const countEl = document.getElementById('icon-count');

            try {
                const response = await fetch(`/api/icons/search?q=${encodeURIComponent(query)}`);
                const data = await response.json();

                countEl.textContent = `${data.total} icons found`;
                this._renderIconGrid(data.icons || []);
            } catch (error) {
                console.error('Failed to search icons:', error);
            }
        },

        // Private: Render icon grid
        _renderIconGrid: function(icons) {
            const grid = document.getElementById('icon-grid');
            const cdnBase = 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg';

            if (icons.length === 0) {
                grid.innerHTML = '<div class="icon-picker-loading">No icons found</div>';
                return;
            }

            // Limit to first 100 for performance
            const displayIcons = icons.slice(0, 100);
            const self = this;

            grid.innerHTML = displayIcons.map(icon => `
                <div class="icon-picker-item" data-icon="${escapeHtml(icon)}" title="${escapeHtml(icon)}">
                    <img src="${cdnBase}/${icon}.svg" alt="${escapeHtml(icon)}" loading="lazy"
                        onerror="this.style.display='none'">
                    <span>${escapeHtml(icon.length > 12 ? icon.substring(0, 10) + '...' : icon)}</span>
                </div>
            `).join('');

            // Add click handlers
            grid.querySelectorAll('.icon-picker-item').forEach(item => {
                item.addEventListener('click', () => {
                    self._selectIcon(item.dataset.icon);
                });
            });

            if (icons.length > 100) {
                grid.innerHTML += `<div class="icon-grid-message">
                    Showing first 100 of ${icons.length} results. Use search to narrow down.
                </div>`;
            }
        },

        // Private: Select an icon
        _selectIcon: async function(iconName) {
            const { sectionIndex, itemIndex, mode } = state;

            // Validate we have proper state
            if (mode === 'service' && (sectionIndex === null || itemIndex === null)) {
                this.close();
                return;
            }
            if (mode !== 'service' && mode !== 'favicon') {
                this.close();
                return;
            }

            // Show loading state
            const grid = document.getElementById('icon-grid');
            const selectedItem = grid.querySelector(`[data-icon="${iconName}"]`);
            if (selectedItem) {
                selectedItem.classList.add('selected');
            }

            try {
                // Download the icon
                const response = await fetch('/api/icons/download', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name: iconName, format: 'svg' })
                });

                if (!response.ok) {
                    throw new Error('Failed to download icon');
                }

                const result = await response.json();
                const iconPath = result.path;

                if (mode === 'favicon') {
                    if (callbacks.onFaviconSelected) {
                        callbacks.onFaviconSelected(iconPath, iconName);
                    }
                } else {
                    if (callbacks.onServiceIconSelected) {
                        callbacks.onServiceIconSelected(sectionIndex, itemIndex, iconPath, iconName);
                    }
                }

                this.close();
            } catch (error) {
                console.error('Failed to select icon:', error);
                alert('Failed to download icon. Please try again.');
                if (selectedItem) {
                    selectedItem.classList.remove('selected');
                }
            }
        },

        // Private: Initialize icon upload
        _initializeIconUpload: function() {
            const dropzone = document.getElementById('icon-upload-dropzone');
            const fileInput = document.getElementById('icon-upload-input');

            if (!dropzone || !fileInput) return;

            const self = this;

            // Remove existing listeners by cloning
            const newDropzone = dropzone.cloneNode(true);
            dropzone.parentNode.replaceChild(newDropzone, dropzone);

            const newFileInput = fileInput.cloneNode(true);
            fileInput.parentNode.replaceChild(newFileInput, fileInput);

            // Re-get references
            const dz = document.getElementById('icon-upload-dropzone');
            const fi = document.getElementById('icon-upload-input');

            // Drag and drop events
            dz.addEventListener('dragover', function(e) {
                e.preventDefault();
                e.stopPropagation();
                dz.classList.add('dragover');
            });

            dz.addEventListener('dragleave', function(e) {
                e.preventDefault();
                e.stopPropagation();
                dz.classList.remove('dragover');
            });

            dz.addEventListener('drop', function(e) {
                e.preventDefault();
                e.stopPropagation();
                dz.classList.remove('dragover');

                const files = e.dataTransfer.files;
                if (files.length > 0) {
                    self._handleIconUpload(files[0]);
                }
            });

            // File input change event
            fi.addEventListener('change', function() {
                if (this.files.length > 0) {
                    self._handleIconUpload(this.files[0]);
                    this.value = ''; // Reset input for next upload
                }
            });
        },

        // Private: Handle icon upload
        _handleIconUpload: async function(file) {
            const { sectionIndex, itemIndex, mode } = state;

            // Validate we have proper state
            if (mode === 'service' && (sectionIndex === null || itemIndex === null)) {
                alert('Please select a service first');
                return;
            }
            if (mode !== 'service' && mode !== 'favicon') {
                alert('Invalid icon picker state');
                return;
            }

            // Validate file type
            const allowedTypes = ['image/svg+xml', 'image/png', 'image/jpeg', 'image/gif', 'image/webp'];
            const allowedExts = ['.svg', '.png', '.jpg', '.jpeg', '.gif', '.webp'];
            const ext = '.' + file.name.split('.').pop().toLowerCase();

            if (!allowedTypes.includes(file.type) && !allowedExts.includes(ext)) {
                alert('Invalid file type. Allowed: SVG, PNG, JPG, GIF, WebP');
                return;
            }

            // Validate file size (5MB max)
            if (file.size > 5 * 1024 * 1024) {
                alert('File too large. Maximum size is 5MB.');
                return;
            }

            // Show uploading state
            const dropzone = document.getElementById('icon-upload-dropzone');
            const originalContent = dropzone.innerHTML;
            dropzone.innerHTML = '<span class="upload-text">Uploading...</span>';

            try {
                const formData = new FormData();
                formData.append('icon', file);

                const response = await fetch('/api/icons/upload', {
                    method: 'POST',
                    body: formData
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(errorText || 'Upload failed');
                }

                const result = await response.json();
                const iconPath = result.path;

                if (mode === 'favicon') {
                    if (callbacks.onFaviconSelected) {
                        callbacks.onFaviconSelected(iconPath, '');
                    }
                } else {
                    if (callbacks.onServiceIconSelected) {
                        callbacks.onServiceIconSelected(sectionIndex, itemIndex, iconPath, '');
                    }
                }

                this.close();
            } catch (error) {
                console.error('Failed to upload icon:', error);
                alert('Failed to upload icon: ' + (error.message || 'Unknown error'));
                dropzone.innerHTML = originalContent;
            }
        }
    };
})();

// Favicon helper functions
function updateFaviconPreview(iconPath) {
    const preview = document.getElementById('favicon-preview');
    if (preview) {
        if (iconPath) {
            preview.innerHTML = `<img src="${escapeHtml(iconPath)}" alt="Favicon" class="favicon-img">`;
        } else {
            preview.innerHTML = '<span class="favicon-placeholder">🌐</span>';
        }
    }
    // Re-render the General section to show/hide clear button
    updateFaviconClearButton(!!iconPath);
}

function updateFaviconClearButton(hasFavicon) {
    const picker = document.querySelector('.favicon-picker');
    if (!picker) return;

    // Remove existing clear button if any
    const existingClearBtn = picker.querySelector('.favicon-clear-btn');
    if (existingClearBtn) {
        existingClearBtn.remove();
    }

    // Add clear button if there's a favicon
    if (hasFavicon) {
        const clearBtn = document.createElement('button');
        clearBtn.type = 'button';
        clearBtn.className = 'button favicon-clear-btn';
        clearBtn.textContent = 'Clear';
        clearBtn.onclick = clearFavicon;
        picker.appendChild(clearBtn);
    }
}

// clearFavicon will be overridden by settings.js to access currentConfig
let clearFavicon = function() {};
