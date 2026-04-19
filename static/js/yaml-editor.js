// YAML Editor functionality for main page
let currentConfig = null;

// Initialize editor enhancements when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    const editor = document.getElementById('yamlEditor');
    const lineNumbers = document.getElementById('yamlLineNumbers');

    if (editor && lineNumbers) {
        // Handle tab key to insert spaces
        editor.addEventListener('keydown', function(e) {
            if (e.key === 'Tab') {
                e.preventDefault();
                const start = this.selectionStart;
                const end = this.selectionEnd;
                const spaces = '    '; // 4 spaces

                if (e.shiftKey) {
                    // Shift+Tab: remove indent from current line
                    const lineStart = this.value.lastIndexOf('\n', start - 1) + 1;
                    const lineContent = this.value.substring(lineStart, start);
                    const spacesToRemove = Math.min(4, lineContent.match(/^ */)[0].length);
                    if (spacesToRemove > 0) {
                        this.value = this.value.substring(0, lineStart) +
                            this.value.substring(lineStart + spacesToRemove);
                        this.selectionStart = this.selectionEnd = start - spacesToRemove;
                    }
                } else {
                    // Tab: insert 4 spaces
                    this.value = this.value.substring(0, start) + spaces + this.value.substring(end);
                    this.selectionStart = this.selectionEnd = start + spaces.length;
                }
                updateLineNumbers();
            }
        });

        // Update line numbers on input
        editor.addEventListener('input', updateLineNumbers);

        // Sync scroll between editor and line numbers
        editor.addEventListener('scroll', function() {
            lineNumbers.scrollTop = this.scrollTop;
        });
    }
});

function updateLineNumbers() {
    const editor = document.getElementById('yamlEditor');
    const lineNumbers = document.getElementById('yamlLineNumbers');
    if (!editor || !lineNumbers) return;

    const lines = editor.value.split('\n').length;
    let html = '';
    for (let i = 1; i <= lines; i++) {
        html += i + '\n';
    }
    lineNumbers.textContent = html;
    lineNumbers.scrollTop = editor.scrollTop;
}

async function loadConfig() {
    try {
        const response = await fetch('/api/config');
        currentConfig = await response.json();
    } catch (error) {
        console.error('Failed to load config:', error);
    }
}

function configToYaml(config) {
    let yaml = jsyaml.dump(config, {
        indent: 4,
        lineWidth: -1,
        quotingType: "'",
        forceQuotes: false
    });

    // Fix array formatting: put first key on same line as dash,
    // and reduce indent of subsequent properties to align
    const lines = yaml.split('\n');
    const result = [];
    const adjustStack = [];

    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        if (!line.trim()) {
            result.push(line);
            continue;
        }

        const spaces = (line.match(/^(\s*)/) || ['', ''])[1].length;

        // Pop adjustments that no longer apply
        while (adjustStack.length > 0 && spaces < adjustStack[adjustStack.length - 1]) {
            adjustStack.pop();
        }

        const totalAdjust = adjustStack.length * 2;

        // Check for standalone dash line
        const dashMatch = line.match(/^(\s*)-\s*$/);
        if (dashMatch && i + 1 < lines.length && lines[i + 1].trim()) {
            const nextLine = lines[i + 1];
            const nextSpaces = (nextLine.match(/^(\s*)/) || ['', ''])[1].length;
            const newIndent = spaces - totalAdjust;
            result.push(' '.repeat(newIndent) + '- ' + nextLine.trim());
            adjustStack.push(nextSpaces);
            i++;
        } else {
            const newIndent = Math.max(0, spaces - totalAdjust);
            result.push(' '.repeat(newIndent) + line.trim());
        }
    }

    return result.join('\n');
}

function yamlToConfig(yamlStr) {
    const parsed = jsyaml.load(yamlStr);
    const config = {
        title: parsed.title || '',
        subtitle: parsed.subtitle || '',
        theme: {
            primary_color: parsed.theme?.primary_color || '',
            secondary_color: parsed.theme?.secondary_color || parsed.theme?.primary_color || '',
            background_color: parsed.theme?.background_color || '',
            card_color: parsed.theme?.card_color || '',
            text_color: parsed.theme?.text_color || '',
            font_family: parsed.theme?.font_family || '',
            favicon: parsed.theme?.favicon || '',
            favicon_name: parsed.theme?.favicon_name || ''
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
                icon_name: item.icon_name || '',
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
    updateLineNumbers();
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
            const errorText = await response.text();
            throw new Error(errorText || 'Save failed');
        }
    } catch (error) {
        statusEl.textContent = 'Error: ' + error.message;
        statusEl.style.color = '#e74c3c';
        console.error('Save YAML error:', error);
    }
}
