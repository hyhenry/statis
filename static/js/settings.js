// Settings page functionality

let currentConfig = null;

document.addEventListener('DOMContentLoaded', async function() {
    await loadConfig();
    renderServicesEditor();
    renderWidgetsEditor();
    initializeFontPicker();
    initializeLayoutSlider();
});

async function loadConfig() {
    try {
        const response = await fetch('/api/config');
        currentConfig = await response.json();
    } catch (error) {
        console.error('Failed to load config:', error);
        alert('Failed to load configuration');
    }
}

function initializeLayoutSlider() {
    const slider = document.getElementById('layoutSlider');
    const widgetLabel = document.getElementById('widgetColsLabel');
    const serviceLabel = document.getElementById('serviceColsLabel');
    const cardsSlider = document.getElementById('cardsPerRowSlider');
    const cardsLabel = document.getElementById('cardsPerRowLabel');

    // Ensure layout exists in config
    if (!currentConfig.layout) {
        currentConfig.layout = { widget_columns: 4, service_columns: 8, cards_per_row: 3 };
    }

    // Column distribution slider
    if (slider) {
        slider.value = currentConfig.layout.widget_columns || 4;
        updateLayoutLabels();

        slider.addEventListener('input', updateLayoutLabels);
        slider.addEventListener('change', function() {
            currentConfig.layout.widget_columns = parseInt(this.value);
            currentConfig.layout.service_columns = 12 - parseInt(this.value);
        });
    }

    // Cards per row slider
    if (cardsSlider) {
        cardsSlider.value = currentConfig.layout.cards_per_row || 3;
        if (cardsLabel) cardsLabel.textContent = cardsSlider.value;

        cardsSlider.addEventListener('input', function() {
            if (cardsLabel) cardsLabel.textContent = this.value;
        });
        cardsSlider.addEventListener('change', function() {
            currentConfig.layout.cards_per_row = parseInt(this.value);
        });
    }

    function updateLayoutLabels() {
        if (slider && widgetLabel && serviceLabel) {
            const widgetCols = parseInt(slider.value);
            widgetLabel.textContent = widgetCols;
            serviceLabel.textContent = 12 - widgetCols;
        }
    }
}

function initializeFontPicker() {
    const fontSelect = document.getElementById('fontFamily');
    const customFontInput = document.getElementById('customFont');

    // Check if current font is a custom one
    const currentFont = currentConfig.theme.font_family || 'system';
    const predefinedFonts = ['system', 'Arial', 'Helvetica', 'Georgia', 'Times New Roman', 'Courier New', 'Verdana', 'Trebuchet MS', 'Impact'];

    if (!predefinedFonts.includes(currentFont)) {
        // It's a custom font
        fontSelect.value = 'custom';
        customFontInput.value = currentFont;
        customFontInput.disabled = false;
    } else {
        customFontInput.disabled = true;
    }

    // Add event listener to enable/disable custom font input
    fontSelect.addEventListener('change', function() {
        if (this.value === 'custom') {
            customFontInput.disabled = false;
            customFontInput.focus();
        } else {
            customFontInput.disabled = true;
            customFontInput.value = '';
        }
    });
}

function renderServicesEditor() {
    const container = document.getElementById('services-editor');
    container.innerHTML = '';

    currentConfig.services.forEach((section, sectionIndex) => {
        const sectionEl = document.createElement('div');
        sectionEl.className = 'section-editor-card';
        sectionEl.innerHTML = `
            <div class="section-editor-header">
                <input type="text" value="${escapeHtml(section.name)}"
                    placeholder="Section Name"
                    onchange="updateSectionName(${sectionIndex}, this.value)"
                    style="margin: 0; font-weight: bold;">
                <button class="button remove-btn" onclick="removeSection(${sectionIndex})">Remove</button>
            </div>
            <div class="section-items-scroll" id="section-${sectionIndex}-items">
                ${section.items.map((item, itemIndex) => renderItemEditor(sectionIndex, itemIndex, item)).join('')}
            </div>
            <button class="button add-item-btn" onclick="addItem(${sectionIndex})">+ Add Service</button>
        `;
        container.appendChild(sectionEl);
    });
}

function renderItemEditor(sectionIndex, itemIndex, item) {
    const iconPreview = item.icon
        ? `<img src="${escapeHtml(item.icon)}" class="icon-preview" alt="icon">`
        : '';
    return `
        <div class="service-item-editor" style="background: rgba(0,0,0,0.2); padding: 1rem; border-radius: 4px; margin-bottom: 1rem;">
            <div class="row">
                <div class="three columns">
                    <label>Name</label>
                    <input type="text" class="u-full-width" value="${escapeHtml(item.name)}"
                        onchange="updateItem(${sectionIndex}, ${itemIndex}, 'name', this.value)">
                </div>
                <div class="three columns">
                    <label>URL</label>
                    <input type="url" class="u-full-width" value="${escapeHtml(item.url)}"
                        onchange="updateItem(${sectionIndex}, ${itemIndex}, 'url', this.value)">
                </div>
                <div class="five columns">
                    <label>Description</label>
                    <input type="text" class="u-full-width" value="${escapeHtml(item.description || '')}"
                        onchange="updateItem(${sectionIndex}, ${itemIndex}, 'description', this.value)">
                </div>
                <div class="one column">
                    <label>&nbsp;</label>
                    <button class="button remove-btn u-full-width" onclick="removeItem(${sectionIndex}, ${itemIndex})">×</button>
                </div>
            </div>
            <div class="row">
                <div class="two columns">
                    <label>Icon (emoji)</label>
                    <input type="text" class="u-full-width" value="${escapeHtml(item.icon_text || '')}"
                        onchange="updateItem(${sectionIndex}, ${itemIndex}, 'icon_text', this.value)">
                </div>
                <div class="ten columns">
                    <label>Icon Image</label>
                    <div class="icon-input-group">
                        ${iconPreview}
                        <input type="text" id="icon-${sectionIndex}-${itemIndex}" value="${escapeHtml(item.icon || '')}"
                            onchange="updateItem(${sectionIndex}, ${itemIndex}, 'icon', this.value)" placeholder="Icon path or URL">
                        <button class="button" onclick="openIconPicker(${sectionIndex}, ${itemIndex})">Browse</button>
                    </div>
                </div>
            </div>
        </div>
    `;
}

function renderWidgetsEditor() {
    const container = document.getElementById('widgets-editor');
    container.innerHTML = '';

    currentConfig.widgets.forEach((widget, index) => {
        const widgetEl = document.createElement('div');
        widgetEl.className = 'widget-editor-card';

        const configPairs = Object.entries(widget.config || {})
            .map(([k, v]) => `${k}: ${v}`)
            .join('\n');

        const isHeader = widget.type === 'header';
        widgetEl.innerHTML = `
            <div class="widget-editor-header">
                <h5>${escapeHtml(widget.title || 'Widget')}</h5>
                <button class="button remove-btn" onclick="removeWidget(${index})">Remove</button>
            </div>
            <div class="row">
                <div class="four columns">
                    <label>Type</label>
                    <select class="u-full-width" onchange="updateWidget(${index}, 'type', this.value)">
                        <option value="uptime-kuma" ${widget.type === 'uptime-kuma' ? 'selected' : ''}>Uptime Kuma</option>
                        <option value="iframe" ${widget.type === 'iframe' ? 'selected' : ''}>iFrame</option>
                        <option value="clock" ${widget.type === 'clock' ? 'selected' : ''}>Clock</option>
                        <option value="system-stats" ${widget.type === 'system-stats' ? 'selected' : ''}>System Stats</option>
                        <option value="rss" ${widget.type === 'rss' ? 'selected' : ''}>RSS Feed</option>
                        <option value="header" ${widget.type === 'header' ? 'selected' : ''}>Header</option>
                    </select>
                </div>
                <div class="eight columns">
                    <label>Title</label>
                    <input type="text" class="u-full-width" value="${escapeHtml(widget.title)}"
                        onchange="updateWidget(${index}, 'title', this.value)">
                </div>
            </div>
            <div class="row">
                <div class="twelve columns">
                    <label>Config (key: value, one per line)</label>
                    <textarea class="u-full-width" rows="3"
                        onchange="updateWidgetConfig(${index}, this.value)"
                        placeholder="url: https://example.com&#10;slug: status"
                        ${isHeader ? 'disabled' : ''}>${escapeHtml(configPairs)}</textarea>
                </div>
            </div>
        `;
        container.appendChild(widgetEl);
    });
}

// Section operations
function addSection() {
    currentConfig.services.push({
        name: 'New Section',
        items: []
    });
    renderServicesEditor();
}

function removeSection(index) {
    if (confirm('Remove this section and all its services?')) {
        currentConfig.services.splice(index, 1);
        renderServicesEditor();
        }
}

function updateSectionName(index, value) {
    currentConfig.services[index].name = value;
}

// Item operations
function addItem(sectionIndex) {
    currentConfig.services[sectionIndex].items.push({
        name: 'New Service',
        url: 'https://',
        icon: '',
        icon_name: '',
        icon_text: '🔗',
        description: '',
        target: ''
    });
    renderServicesEditor();
}

function removeItem(sectionIndex, itemIndex) {
    currentConfig.services[sectionIndex].items.splice(itemIndex, 1);
    renderServicesEditor();
}

function updateItem(sectionIndex, itemIndex, field, value) {
    currentConfig.services[sectionIndex].items[itemIndex][field] = value;
    // Clear icon_name when icon is set directly via UI to prevent backend from overriding
    if (field === 'icon') {
        currentConfig.services[sectionIndex].items[itemIndex].icon_name = '';
    }
}

// Widget operations
function addWidget() {
    currentConfig.widgets.push({
        type: 'clock',
        title: 'New Widget',
        config: {}
    });
    renderWidgetsEditor();
}

function removeWidget(index) {
    if (confirm('Remove this widget?')) {
        currentConfig.widgets.splice(index, 1);
        renderWidgetsEditor();
        }
}

function updateWidget(index, field, value) {
    currentConfig.widgets[index][field] = value;
    renderWidgetsEditor();
}

function updateWidgetConfig(index, value) {
    const config = {};
    value.split('\n').forEach(line => {
        const [key, ...rest] = line.split(':');
        if (key && rest.length) {
            config[key.trim()] = rest.join(':').trim();
        }
    });
    currentConfig.widgets[index].config = config;
}

// Theme updates
document.getElementById('title')?.addEventListener('change', (e) => {
    currentConfig.title = e.target.value;
});

document.getElementById('subtitle')?.addEventListener('change', (e) => {
    currentConfig.subtitle = e.target.value;
});

document.getElementById('primaryColor')?.addEventListener('change', (e) => {
    currentConfig.theme.primary_color = e.target.value;
    document.documentElement.style.setProperty('--primary-color', e.target.value);
});

document.getElementById('secondaryColor')?.addEventListener('change', (e) => {
    currentConfig.theme.secondary_color = e.target.value;
    document.documentElement.style.setProperty('--secondary-color', e.target.value);
});

document.getElementById('bgColor')?.addEventListener('change', (e) => {
    currentConfig.theme.background_color = e.target.value;
    document.documentElement.style.setProperty('--bg-color', e.target.value);
});

document.getElementById('cardColor')?.addEventListener('change', (e) => {
    currentConfig.theme.card_color = e.target.value;
    document.documentElement.style.setProperty('--card-color', e.target.value);
});

document.getElementById('textColor')?.addEventListener('change', (e) => {
    currentConfig.theme.text_color = e.target.value;
    document.documentElement.style.setProperty('--text-color', e.target.value);
});

// Save configuration
async function saveConfig() {
    const statusEl = document.getElementById('save-status');

    // Update config from form fields
    currentConfig.title = document.getElementById('title').value;
    currentConfig.subtitle = document.getElementById('subtitle').value;

    // Get font family (either from dropdown or custom input)
    const fontSelect = document.getElementById('fontFamily');
    const customFont = document.getElementById('customFont');
    let fontFamily = fontSelect.value;

    // Validation: if "custom" is selected, custom font field must not be blank
    if (fontSelect.value === 'custom') {
        if (!customFont.value.trim()) {
            statusEl.textContent = '✗ Please enter a custom font name or select a different font';
            statusEl.style.color = '#e74c3c';
            customFont.focus();
            return;
        }
        fontFamily = customFont.value.trim();
    }

    currentConfig.theme = {
        primary_color: document.getElementById('primaryColor').value,
        secondary_color: document.getElementById('secondaryColor').value,
        background_color: document.getElementById('bgColor').value,
        card_color: document.getElementById('cardColor').value,
        text_color: document.getElementById('textColor').value,
        font_family: fontFamily,
        favicon: currentConfig.theme.favicon || '',
        favicon_name: currentConfig.theme.favicon_name || ''
    };

    // Update layout from sliders
    const layoutSlider = document.getElementById('layoutSlider');
    const cardsSlider = document.getElementById('cardsPerRowSlider');
    const widgetCols = layoutSlider ? parseInt(layoutSlider.value) : (currentConfig.layout?.widget_columns || 4);
    const cardsPerRow = cardsSlider ? parseInt(cardsSlider.value) : (currentConfig.layout?.cards_per_row || 3);
    currentConfig.layout = {
        widget_columns: widgetCols,
        service_columns: 12 - widgetCols,
        cards_per_row: cardsPerRow
    };

    try {
        const response = await fetch('/api/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(currentConfig)
        });

        if (response.ok) {
            statusEl.textContent = '✓ Saved!';
            statusEl.style.color = '#2ecc71';
            setTimeout(() => { statusEl.textContent = ''; }, 3000);
        } else {
            const errorText = await response.text();
            throw new Error(errorText || 'Save failed');
        }
    } catch (error) {
        statusEl.textContent = '✗ ' + (error.message || 'Save failed');
        statusEl.style.color = '#e74c3c';
        console.error('Save error:', error);
    }
}

// Clean unused assets (fonts and icons not currently in use)
async function cleanUnusedAssets() {
    const statusEl = document.getElementById('save-status');

    if (!confirm('This will delete any downloaded fonts and icons that are not currently being used. Continue?')) {
        return;
    }

    try {
        const response = await fetch('/api/assets/clean-unused', {
            method: 'DELETE'
        });

        if (response.ok) {
            const result = await response.json();
            const total = result.fonts_removed + result.icons_removed;

            if (total === 0) {
                statusEl.textContent = '✓ No unused assets found';
            } else {
                statusEl.textContent = `✓ Removed ${result.fonts_removed} fonts, ${result.icons_removed} icons`;
            }
            statusEl.style.color = '#2ecc71';
            setTimeout(() => { statusEl.textContent = ''; }, 5000);
        } else {
            throw new Error('Cleanup failed');
        }
    } catch (error) {
        statusEl.textContent = '✗ Cleanup failed';
        statusEl.style.color = '#e74c3c';
        console.error('Clean unused assets error:', error);
    }
}

// Save config silently (without UI feedback)
async function saveConfigSilently() {
    try {
        await fetch('/api/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(currentConfig)
        });
    } catch (error) {
        console.error('Save error:', error);
    }
}

// escapeHtml is provided by utils.js

// Icon Picker
let iconPickerState = {
    sectionIndex: null,
    itemIndex: null,
    mode: null, // 'service' or 'favicon'
    icons: [],
    searchTimeout: null
};

function openIconPicker(sectionIndexOrMode, itemIndex) {
    // Check if opening for favicon (first arg is 'favicon' string)
    if (sectionIndexOrMode === 'favicon') {
        iconPickerState.mode = 'favicon';
        iconPickerState.sectionIndex = null;
        iconPickerState.itemIndex = null;
    } else {
        iconPickerState.mode = 'service';
        iconPickerState.sectionIndex = sectionIndexOrMode;
        iconPickerState.itemIndex = itemIndex;
    }

    const modal = document.getElementById('icon-picker-modal');
    modal.style.display = 'block';

    const searchInput = document.getElementById('icon-search');
    searchInput.value = '';
    searchInput.focus();

    // Load icons if not already loaded
    if (iconPickerState.icons.length === 0) {
        loadIcons();
    } else {
        renderIconGrid(iconPickerState.icons);
    }

    // Setup search with debounce
    searchInput.oninput = function() {
        clearTimeout(iconPickerState.searchTimeout);
        iconPickerState.searchTimeout = setTimeout(() => {
            searchIcons(this.value);
        }, 300);
    };
}

function closeIconPicker() {
    const modal = document.getElementById('icon-picker-modal');
    modal.style.display = 'none';
    iconPickerState.sectionIndex = null;
    iconPickerState.itemIndex = null;
    iconPickerState.mode = null;
}

async function loadIcons() {
    const grid = document.getElementById('icon-grid');
    const countEl = document.getElementById('icon-count');

    grid.innerHTML = '<div class="icon-picker-loading">Loading icons...</div>';

    try {
        const response = await fetch('/api/icons/search');
        const data = await response.json();

        iconPickerState.icons = data.icons || [];
        countEl.textContent = `${iconPickerState.icons.length} icons available`;
        renderIconGrid(iconPickerState.icons);
    } catch (error) {
        console.error('Failed to load icons:', error);
        grid.innerHTML = '<div class="icon-picker-loading">Failed to load icons</div>';
        countEl.textContent = 'Error loading icons';
    }
}

async function searchIcons(query) {
    const grid = document.getElementById('icon-grid');
    const countEl = document.getElementById('icon-count');

    try {
        const response = await fetch(`/api/icons/search?q=${encodeURIComponent(query)}`);
        const data = await response.json();

        countEl.textContent = `${data.total} icons found`;
        renderIconGrid(data.icons || []);
    } catch (error) {
        console.error('Failed to search icons:', error);
    }
}

function renderIconGrid(icons) {
    const grid = document.getElementById('icon-grid');
    const cdnBase = 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg';

    if (icons.length === 0) {
        grid.innerHTML = '<div class="icon-picker-loading">No icons found</div>';
        return;
    }

    // Limit to first 100 for performance
    const displayIcons = icons.slice(0, 100);

    grid.innerHTML = displayIcons.map(icon => `
        <div class="icon-picker-item" onclick="selectIcon('${escapeHtml(icon)}')" title="${escapeHtml(icon)}">
            <img src="${cdnBase}/${icon}.svg" alt="${escapeHtml(icon)}" loading="lazy"
                onerror="this.style.display='none'">
            <span>${escapeHtml(icon.length > 12 ? icon.substring(0, 10) + '...' : icon)}</span>
        </div>
    `).join('');

    if (icons.length > 100) {
        grid.innerHTML += `<div class="icon-picker-loading" style="grid-column: 1/-1; padding: 1rem;">
            Showing first 100 of ${icons.length} results. Use search to narrow down.
        </div>`;
    }
}

async function selectIcon(iconName) {
    const { sectionIndex, itemIndex, mode } = iconPickerState;

    // Validate we have proper state
    if (mode === 'service' && (sectionIndex === null || itemIndex === null)) {
        closeIconPicker();
        return;
    }
    if (mode !== 'service' && mode !== 'favicon') {
        closeIconPicker();
        return;
    }

    // Show loading state
    const grid = document.getElementById('icon-grid');
    const selectedItem = grid.querySelector(`[title="${iconName}"]`);
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
            // Update favicon in config
            currentConfig.theme.favicon = iconPath;
            currentConfig.theme.favicon_name = iconName;

            // Update favicon preview
            updateFaviconPreview(iconPath);
        } else {
            // Update service icon
            currentConfig.services[sectionIndex].items[itemIndex].icon = iconPath;
            currentConfig.services[sectionIndex].items[itemIndex].icon_name = iconName;

            // Update the input field
            const input = document.getElementById(`icon-${sectionIndex}-${itemIndex}`);
            if (input) {
                input.value = iconPath;
            }

            // Re-render to show preview
            renderServicesEditor();
        }

        closeIconPicker();
    } catch (error) {
        console.error('Failed to select icon:', error);
        alert('Failed to download icon. Please try again.');
        if (selectedItem) {
            selectedItem.classList.remove('selected');
        }
    }
}

// Close modal when clicking outside
document.addEventListener('click', function(e) {
    const modal = document.getElementById('icon-picker-modal');
    if (e.target === modal) {
        closeIconPicker();
    }
});

// Close modal on escape key
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        closeIconPicker();
    }
});

// Icon Upload functionality
function initializeIconUpload() {
    const dropzone = document.getElementById('icon-upload-dropzone');
    const fileInput = document.getElementById('icon-upload-input');

    if (!dropzone || !fileInput) return;

    // Drag and drop events
    dropzone.addEventListener('dragover', function(e) {
        e.preventDefault();
        e.stopPropagation();
        dropzone.classList.add('dragover');
    });

    dropzone.addEventListener('dragleave', function(e) {
        e.preventDefault();
        e.stopPropagation();
        dropzone.classList.remove('dragover');
    });

    dropzone.addEventListener('drop', function(e) {
        e.preventDefault();
        e.stopPropagation();
        dropzone.classList.remove('dragover');

        const files = e.dataTransfer.files;
        if (files.length > 0) {
            handleIconUpload(files[0]);
        }
    });

    // File input change event
    fileInput.addEventListener('change', function() {
        if (this.files.length > 0) {
            handleIconUpload(this.files[0]);
            this.value = ''; // Reset input for next upload
        }
    });
}

async function handleIconUpload(file) {
    const { sectionIndex, itemIndex, mode } = iconPickerState;

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
            // Update favicon in config - clear favicon_name since we're uploading directly
            currentConfig.theme.favicon = iconPath;
            currentConfig.theme.favicon_name = '';

            // Update favicon preview
            updateFaviconPreview(iconPath);
        } else {
            // Update the config - clear icon_name since we're setting icon directly via upload
            currentConfig.services[sectionIndex].items[itemIndex].icon = iconPath;
            currentConfig.services[sectionIndex].items[itemIndex].icon_name = '';

            // Update the input field
            const input = document.getElementById(`icon-${sectionIndex}-${itemIndex}`);
            if (input) {
                input.value = iconPath;
            }

            // Re-render to show preview
            renderServicesEditor();
        }

        closeIconPicker();
    } catch (error) {
        console.error('Failed to upload icon:', error);
        alert('Failed to upload icon: ' + (error.message || 'Unknown error'));
        dropzone.innerHTML = originalContent;
    }
}

// Initialize upload when modal opens
const originalOpenIconPicker = openIconPicker;
openIconPicker = function(sectionIndexOrMode, itemIndex) {
    originalOpenIconPicker(sectionIndexOrMode, itemIndex);
    // Re-initialize upload handlers each time modal opens
    setTimeout(initializeIconUpload, 0);
};

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

function clearFavicon() {
    currentConfig.theme.favicon = '';
    currentConfig.theme.favicon_name = '';
    updateFaviconPreview('');
}
