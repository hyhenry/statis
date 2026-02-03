// Settings page functionality

let currentConfig = null;

document.addEventListener('DOMContentLoaded', async function() {
    await loadConfig();
    renderServicesEditor();
    renderWidgetsEditor();
    updateYamlEditor();
    initializeFontPicker();
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
        sectionEl.className = 'editor-item';
        sectionEl.innerHTML = `
            <div class="editor-item-header">
                <input type="text" value="${escapeHtml(section.name)}" 
                    placeholder="Section Name" 
                    onchange="updateSectionName(${sectionIndex}, this.value)"
                    style="margin: 0; font-weight: bold;">
                <button class="button remove-btn" onclick="removeSection(${sectionIndex})">Remove</button>
            </div>
            <div id="section-${sectionIndex}-items">
                ${section.items.map((item, itemIndex) => renderItemEditor(sectionIndex, itemIndex, item)).join('')}
            </div>
            <button class="button add-item-btn" onclick="addItem(${sectionIndex})">+ Add Service</button>
        `;
        container.appendChild(sectionEl);
    });
}

function renderItemEditor(sectionIndex, itemIndex, item) {
    return `
        <div class="row" style="background: rgba(0,0,0,0.2); padding: 1rem; border-radius: 4px; margin-bottom: 1rem;">
            <div class="four columns">
                <label>Name</label>
                <input type="text" class="u-full-width" value="${escapeHtml(item.name)}" 
                    onchange="updateItem(${sectionIndex}, ${itemIndex}, 'name', this.value)">
            </div>
            <div class="four columns">
                <label>URL</label>
                <input type="url" class="u-full-width" value="${escapeHtml(item.url)}" 
                    onchange="updateItem(${sectionIndex}, ${itemIndex}, 'url', this.value)">
            </div>
            <div class="two columns">
                <label>Icon (emoji/text)</label>
                <input type="text" class="u-full-width" value="${escapeHtml(item.icon_text || '')}" 
                    onchange="updateItem(${sectionIndex}, ${itemIndex}, 'icon_text', this.value)">
            </div>
            <div class="two columns">
                <label>&nbsp;</label>
                <button class="button remove-btn u-full-width" onclick="removeItem(${sectionIndex}, ${itemIndex})">×</button>
            </div>
            <div class="six columns">
                <label>Description</label>
                <input type="text" class="u-full-width" value="${escapeHtml(item.description || '')}" 
                    onchange="updateItem(${sectionIndex}, ${itemIndex}, 'description', this.value)">
            </div>
            <div class="six columns">
                <label>Icon URL (optional, overrides emoji)</label>
                <input type="url" class="u-full-width" value="${escapeHtml(item.icon || '')}" 
                    onchange="updateItem(${sectionIndex}, ${itemIndex}, 'icon', this.value)">
            </div>
        </div>
    `;
}

function renderWidgetsEditor() {
    const container = document.getElementById('widgets-editor');
    container.innerHTML = '';

    currentConfig.widgets.forEach((widget, index) => {
        const widgetEl = document.createElement('div');
        widgetEl.className = 'editor-item';
        
        const configPairs = Object.entries(widget.config || {})
            .map(([k, v]) => `${k}: ${v}`)
            .join('\n');

        widgetEl.innerHTML = `
            <div class="editor-item-header">
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
                        placeholder="url: https://example.com&#10;slug: status">${escapeHtml(configPairs)}</textarea>
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
    updateYamlEditor();
}

function removeSection(index) {
    if (confirm('Remove this section and all its services?')) {
        currentConfig.services.splice(index, 1);
        renderServicesEditor();
        updateYamlEditor();
    }
}

function updateSectionName(index, value) {
    currentConfig.services[index].name = value;
    updateYamlEditor();
}

// Item operations
function addItem(sectionIndex) {
    currentConfig.services[sectionIndex].items.push({
        name: 'New Service',
        url: 'https://',
        icon_text: '🔗',
        description: ''
    });
    renderServicesEditor();
    updateYamlEditor();
}

function removeItem(sectionIndex, itemIndex) {
    currentConfig.services[sectionIndex].items.splice(itemIndex, 1);
    renderServicesEditor();
    updateYamlEditor();
}

function updateItem(sectionIndex, itemIndex, field, value) {
    currentConfig.services[sectionIndex].items[itemIndex][field] = value;
    updateYamlEditor();
}

// Widget operations
function addWidget() {
    currentConfig.widgets.push({
        type: 'clock',
        title: 'New Widget',
        config: {}
    });
    renderWidgetsEditor();
    updateYamlEditor();
}

function removeWidget(index) {
    if (confirm('Remove this widget?')) {
        currentConfig.widgets.splice(index, 1);
        renderWidgetsEditor();
        updateYamlEditor();
    }
}

function updateWidget(index, field, value) {
    currentConfig.widgets[index][field] = value;
    renderWidgetsEditor();
    updateYamlEditor();
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
    updateYamlEditor();
}

// Theme updates
document.getElementById('title')?.addEventListener('change', (e) => {
    currentConfig.title = e.target.value;
    updateYamlEditor();
});

document.getElementById('subtitle')?.addEventListener('change', (e) => {
    currentConfig.subtitle = e.target.value;
    updateYamlEditor();
});

document.getElementById('primaryColor')?.addEventListener('change', (e) => {
    currentConfig.theme.primary_color = e.target.value;
    document.documentElement.style.setProperty('--primary-color', e.target.value);
    updateYamlEditor();
});

document.getElementById('bgColor')?.addEventListener('change', (e) => {
    currentConfig.theme.background_color = e.target.value;
    document.documentElement.style.setProperty('--bg-color', e.target.value);
    updateYamlEditor();
});

document.getElementById('cardColor')?.addEventListener('change', (e) => {
    currentConfig.theme.card_color = e.target.value;
    document.documentElement.style.setProperty('--card-color', e.target.value);
    updateYamlEditor();
});

document.getElementById('textColor')?.addEventListener('change', (e) => {
    currentConfig.theme.text_color = e.target.value;
    document.documentElement.style.setProperty('--text-color', e.target.value);
    updateYamlEditor();
});

// YAML editor
function updateYamlEditor() {
    const yaml = configToYaml(currentConfig);
    document.getElementById('yamlEditor').value = yaml;
}

function loadFromYaml() {
    try {
        const yaml = document.getElementById('yamlEditor').value;
        currentConfig = yamlToConfig(yaml);
        
        // Update form fields
        document.getElementById('title').value = currentConfig.title;
        document.getElementById('subtitle').value = currentConfig.subtitle;
        document.getElementById('primaryColor').value = currentConfig.theme.primary_color;
        document.getElementById('bgColor').value = currentConfig.theme.background_color;
        document.getElementById('cardColor').value = currentConfig.theme.card_color;
        document.getElementById('textColor').value = currentConfig.theme.text_color;
        
        renderServicesEditor();
        renderWidgetsEditor();
        
        alert('Configuration loaded from YAML');
    } catch (error) {
        alert('Invalid YAML: ' + error.message);
    }
}

// Simple YAML serializer (for display purposes)
function configToYaml(config) {
    let yaml = '';
    yaml += `title: "${config.title}"\n`;
    yaml += `subtitle: "${config.subtitle}"\n\n`;
    
    yaml += `theme:\n`;
    yaml += `  primary_color: "${config.theme.primary_color}"\n`;
    yaml += `  background_color: "${config.theme.background_color}"\n`;
    yaml += `  card_color: "${config.theme.card_color}"\n`;
    yaml += `  text_color: "${config.theme.text_color}"\n\n`;
    
    yaml += `services:\n`;
    config.services.forEach(section => {
        yaml += `  - name: "${section.name}"\n`;
        yaml += `    items:\n`;
        section.items.forEach(item => {
            yaml += `      - name: "${item.name}"\n`;
            yaml += `        url: "${item.url}"\n`;
            if (item.icon) yaml += `        icon: "${item.icon}"\n`;
            if (item.icon_text) yaml += `        icon_text: "${item.icon_text}"\n`;
            if (item.description) yaml += `        description: "${item.description}"\n`;
        });
    });
    
    yaml += `\nwidgets:\n`;
    config.widgets.forEach(widget => {
        yaml += `  - type: "${widget.type}"\n`;
        yaml += `    title: "${widget.title}"\n`;
        yaml += `    config:\n`;
        Object.entries(widget.config || {}).forEach(([k, v]) => {
            yaml += `      ${k}: "${v}"\n`;
        });
    });
    
    return yaml;
}

// Simple YAML parser (basic implementation)
function yamlToConfig(yaml) {
    // For a real implementation, use js-yaml library
    // This is a simplified version that works with our specific format
    const lines = yaml.split('\n');
    const config = {
        title: '',
        subtitle: '',
        theme: {},
        services: [],
        widgets: []
    };
    
    let currentSection = null;
    let currentItem = null;
    let currentWidget = null;
    
    lines.forEach(line => {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) return;
        
        const match = trimmed.match(/^(\w+):\s*"?([^"]*)"?$/);
        if (match) {
            const [, key, value] = match;
            
            if (key === 'title') config.title = value;
            else if (key === 'subtitle') config.subtitle = value;
            else if (currentWidget && key !== 'type' && key !== 'title') {
                currentWidget.config[key] = value;
            }
            else if (currentItem) {
                currentItem[key] = value;
            }
            else if (line.startsWith('    ') && currentSection) {
                currentSection[key] = value;
            }
            else if (line.startsWith('  ') && currentSection === null) {
                config.theme[key] = value;
            }
        }
        
        if (trimmed === 'theme:') currentSection = null;
        if (trimmed === 'services:') { currentSection = 'services'; currentItem = null; }
        if (trimmed === 'widgets:') { currentSection = 'widgets'; currentWidget = null; }
        
        if (trimmed.startsWith('- name:') && currentSection === 'services') {
            const name = trimmed.match(/"([^"]*)"/)?.[1] || '';
            currentItem = { name, items: [] };
            config.services.push(currentItem);
        }
        
        if (trimmed.startsWith('- type:') && currentSection === 'widgets') {
            const type = trimmed.match(/"([^"]*)"/)?.[1] || '';
            currentWidget = { type, title: '', config: {} };
            config.widgets.push(currentWidget);
        }
    });
    
    return config;
}

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
        background_color: document.getElementById('bgColor').value,
        card_color: document.getElementById('cardColor').value,
        text_color: document.getElementById('textColor').value,
        font_family: fontFamily
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
            throw new Error('Save failed');
        }
    } catch (error) {
        statusEl.textContent = '✗ Save failed';
        statusEl.style.color = '#e74c3c';
        console.error('Save error:', error);
    }
}

// Clear all custom fonts
async function clearFonts() {
    const statusEl = document.getElementById('clear-fonts-status');

    if (!confirm('This will delete all downloaded custom fonts and reset to system default. Continue?')) {
        return;
    }

    try {
        const response = await fetch('/api/fonts/clear', {
            method: 'DELETE'
        });

        if (response.ok) {
            // Reset font selection to system
            const fontSelect = document.getElementById('fontFamily');
            const customFont = document.getElementById('customFont');

            fontSelect.value = 'system';
            customFont.value = '';
            customFont.disabled = true;

            // Update current config
            currentConfig.theme.font_family = 'system';
            updateYamlEditor();

            // Save the updated config
            await saveConfigSilently();

            statusEl.textContent = '✓ Fonts cleared';
            statusEl.style.color = '#2ecc71';
            setTimeout(() => { statusEl.textContent = ''; }, 3000);
        } else {
            throw new Error('Clear failed');
        }
    } catch (error) {
        statusEl.textContent = '✗ Clear failed';
        statusEl.style.color = '#e74c3c';
        console.error('Clear fonts error:', error);
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

// Utility function
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
