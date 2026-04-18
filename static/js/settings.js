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
                    class="section-name-input">
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
        <div class="service-item-editor">
            <div class="row">
                <div class="three columns">
                    <label>Name</label>
                    <input type="text" class="u-full-width" value="${escapeHtml(item.name)}"
                        onchange="updateItem(${sectionIndex}, ${itemIndex}, 'name', this.value)">
                </div>
                <div class="four columns">
                    <label>URL</label>
                    <input type="url" class="u-full-width" value="${escapeHtml(item.url)}"
                        onchange="updateItem(${sectionIndex}, ${itemIndex}, 'url', this.value)">
                </div>
                <div class="four columns">
                    <label>Description</label>
                    <input type="text" class="u-full-width" value="${escapeHtml(item.description || '')}"
                        onchange="updateItem(${sectionIndex}, ${itemIndex}, 'description', this.value)">
                </div>
                <div class="one column">
                    <label>&nbsp;</label>
                    <button class="header-icon-btn" onclick="removeItem(${sectionIndex}, ${itemIndex})">❌</button>
                </div>
            </div>
            <div class="row">
                <div class="three columns">
                    <label>Icon (emoji)</label>
                    <input type="text" class="u-full-width" value="${escapeHtml(item.icon_text || '')}"
                        onchange="updateItem(${sectionIndex}, ${itemIndex}, 'icon_text', this.value)">
                </div>
                <div class="nine columns">
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

        const template = widgetConfigTemplates[widget.type] || { hasConfig: true, placeholder: '' };
        const hasConfig = template.hasConfig;
        const placeholder = template.placeholder || '';

        widgetEl.innerHTML = `
            <div class="widget-editor-header">
                <h5>${escapeHtml(widget.title || 'Widget')}</h5>
                <button class="button remove-btn" onclick="removeWidget(${index})">Remove</button>
            </div>
            <div class="row">
                <div class="four columns">
                    <label>Type</label>
                    <select class="u-full-width" onchange="updateWidget(${index}, 'type', this.value)">
                        <option value="clock" ${widget.type === 'clock' ? 'selected' : ''}>Clock</option>
                        <option value="uptime-kuma" ${widget.type === 'uptime-kuma' ? 'selected' : ''}>Uptime Kuma</option>
                        <option value="rss" ${widget.type === 'rss' ? 'selected' : ''}>RSS Feed</option>
                        <option value="iframe" ${widget.type === 'iframe' ? 'selected' : ''}>iFrame</option>
                        <option value="header" ${widget.type === 'header' ? 'selected' : ''}>Header</option>
                        <option value="system-stats" ${widget.type === 'system-stats' ? 'selected' : ''}>System Stats</option>
                        <option value="truenas-scale" ${widget.type === 'truenas-scale' ? 'selected' : ''}>TrueNAS SCALE</option>
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
                    <label>Config (key: value, one per line)${!hasConfig ? ' - No configuration needed' : ''}</label>
                    <textarea class="u-full-width" rows="3"
                        onchange="updateWidgetConfig(${index}, this.value)"
                        placeholder="${escapeHtml(placeholder)}"
                        ${!hasConfig ? 'disabled' : ''}>${escapeHtml(configPairs)}</textarea>
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

// Widget type configurations
const widgetConfigTemplates = {
    'header': {
        hasConfig: false,
        defaults: {}
    },
    'clock': {
        hasConfig: false,
        defaults: {}
    },
    'uptime-kuma': {
        hasConfig: true,
        defaults: {
            url: 'http://uptime-kuma:3001',
            slug: 'status'
        },
        placeholder: 'url: http://uptime-kuma:3001\nslug: status'
    },
    'rss': {
        hasConfig: true,
        defaults: {
            url: 'https://example.com/feed.xml',
            items_per_page: '3',
            refresh: '300'
        },
        placeholder: 'url: https://example.com/feed.xml\nitems_per_page: 3\nrefresh: 300'
    },
    'iframe': {
        hasConfig: true,
        defaults: {
            url: 'https://example.com',
            height: '400px'
        },
        placeholder: 'url: https://example.com\nheight: 400px'
    },
    'system-stats': {
        hasConfig: false,
        defaults: {}
    },
    'truenas-scale': {
        hasConfig: true,
        defaults: {
            url: 'https://truenas.local',
            api_key: '',
            show_system: 'true',
            show_pools: 'true',
            show_disks: 'true',
            show_backups: 'true'
        },
        placeholder: 'url: https://truenas.local\napi_key: 1-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\nshow_system: true\nshow_pools: true\nshow_disks: true\nshow_backups: true'
    }
};

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

    // When changing widget type, update config to appropriate defaults
    if (field === 'type') {
        const template = widgetConfigTemplates[value];
        if (template) {
            if (template.hasConfig) {
                // Set default config for widgets that need configuration
                currentConfig.widgets[index].config = { ...template.defaults };
            } else {
                // Clear config for widgets that don't need it
                currentConfig.widgets[index].config = {};
            }
        }
    }

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

// Theme updates - data-driven approach for color inputs
const themeColorInputs = [
    { id: 'primaryColor', prop: 'primary_color', css: '--primary-color' },
    { id: 'secondaryColor', prop: 'secondary_color', css: '--secondary-color' },
    { id: 'bgColor', prop: 'background_color', css: '--bg-color' },
    { id: 'cardColor', prop: 'card_color', css: '--card-color' },
    { id: 'textColor', prop: 'text_color', css: '--text-color' }
];

themeColorInputs.forEach(({ id, prop, css }) => {
    document.getElementById(id)?.addEventListener('change', (e) => {
        currentConfig.theme[prop] = e.target.value;
        document.documentElement.style.setProperty(css, e.target.value);
    });
});

// Title/subtitle updates
document.getElementById('title')?.addEventListener('change', (e) => {
    currentConfig.title = e.target.value;
});

document.getElementById('subtitle')?.addEventListener('change', (e) => {
    currentConfig.subtitle = e.target.value;
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
// IconPicker module is provided by icon-picker.js

// Initialize IconPicker with callbacks
document.addEventListener('DOMContentLoaded', function() {
    IconPicker.init({
        onServiceIconSelected: function(sectionIndex, itemIndex, iconPath, iconName) {
            // Update service icon in config
            currentConfig.services[sectionIndex].items[itemIndex].icon = iconPath;
            currentConfig.services[sectionIndex].items[itemIndex].icon_name = iconName;

            // Update the input field
            const input = document.getElementById(`icon-${sectionIndex}-${itemIndex}`);
            if (input) {
                input.value = iconPath;
            }

            // Re-render to show preview
            renderServicesEditor();
        },
        onFaviconSelected: function(iconPath, iconName) {
            // Update favicon in config
            currentConfig.theme.favicon = iconPath;
            currentConfig.theme.favicon_name = iconName;

            // Update favicon preview
            updateFaviconPreview(iconPath);
        },
        renderServicesEditor: renderServicesEditor
    });

    // Override clearFavicon from icon-picker.js
    clearFavicon = function() {
        currentConfig.theme.favicon = '';
        currentConfig.theme.favicon_name = '';
        updateFaviconPreview('');
    };
});

// Wrapper function for template onclick handlers
function openIconPicker(sectionIndexOrMode, itemIndex) {
    IconPicker.open(sectionIndexOrMode, itemIndex);
}

