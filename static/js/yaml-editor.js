// YAML Editor functionality for main page
let currentConfig = null;

async function loadConfig() {
    try {
        const response = await fetch('/api/config');
        currentConfig = await response.json();
    } catch (error) {
        console.error('Failed to load config:', error);
    }
}

function configToYaml(config) {
    return jsyaml.dump(config, {
        indent: 4,
        lineWidth: -1,
        quotingType: "'",
        forceQuotes: false
    });
}

function yamlToConfig(yaml) {
    const parsed = jsyaml.load(yaml);
    const config = {
        title: parsed.title || '',
        subtitle: parsed.subtitle || '',
        theme: {
            primary_color: parsed.theme?.primary_color || '',
            secondary_color: parsed.theme?.secondary_color || parsed.theme?.primary_color || '',
            background_color: parsed.theme?.background_color || '',
            card_color: parsed.theme?.card_color || '',
            text_color: parsed.theme?.text_color || '',
            font_family: parsed.theme?.font_family || ''
        },
        layout: {
            widget_columns: parsed.layout?.widget_columns || 4,
            service_columns: parsed.layout?.service_columns || 8,
            cards_per_row: parsed.layout?.cards_per_row || 3
        },
        services: [],
        widgets: []
    };

    if (Array.isArray(parsed.services)) {
        config.services = parsed.services.map(section => ({
            name: section.name || '',
            items: Array.isArray(section.items) ? section.items.map(item => ({
                name: item.name || '',
                url: item.url || '',
                icon: item.icon || '',
                icon_text: item.icon_text || '',
                description: item.description || '',
                target: item.target || ''
            })) : []
        }));
    }

    if (Array.isArray(parsed.widgets)) {
        config.widgets = parsed.widgets.map(widget => ({
            type: widget.type || '',
            title: widget.title || '',
            config: widget.config && typeof widget.config === 'object' ? widget.config : {}
        }));
    }

    return config;
}

async function openYamlModal() {
    if (!currentConfig) {
        await loadConfig();
    }
    document.getElementById('yamlEditor').value = configToYaml(currentConfig);
    document.getElementById('yamlModal').classList.add('active');
}

function closeYamlModal() {
    document.getElementById('yamlModal').classList.remove('active');
    document.getElementById('yaml-save-status').textContent = '';
}

async function saveFromYaml() {
    const statusEl = document.getElementById('yaml-save-status');

    try {
        const yaml = document.getElementById('yamlEditor').value;
        const parsedConfig = yamlToConfig(yaml);

        const response = await fetch('/api/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(parsedConfig)
        });

        if (response.ok) {
            currentConfig = parsedConfig;
            statusEl.textContent = 'Saved! Reloading...';
            statusEl.style.color = '#2ecc71';
            setTimeout(() => {
                window.location.reload();
            }, 1000);
        } else {
            throw new Error('Save failed');
        }
    } catch (error) {
        statusEl.textContent = 'Invalid YAML or save failed: ' + error.message;
        statusEl.style.color = '#e74c3c';
        console.error('Save YAML error:', error);
    }
}
