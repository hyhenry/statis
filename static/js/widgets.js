// Widget initialization and handling

document.addEventListener('DOMContentLoaded', function() {
    // Initialize all widgets
    const widgets = document.querySelectorAll('.widget');
    widgets.forEach(initWidget);
});

function initWidget(widgetEl) {
    const type = widgetEl.dataset.type;
    const configStr = widgetEl.dataset.config || '{}';
    
    // Parse config (remove trailing comma if present)
    let config = {};
    try {
        const cleanConfig = '{' + configStr.replace(/,\s*$/, '') + '}';
        config = JSON.parse(cleanConfig);
    } catch (e) {
        console.error('Failed to parse widget config:', e);
    }

    switch (type) {
        case 'uptime-kuma':
            initUptimeKumaWidget(widgetEl, config);
            break;
        case 'iframe':
            initIframeWidget(widgetEl, config);
            break;
        case 'clock':
            initClockWidget(widgetEl, config);
            break;
        case 'system-stats':
            initSystemStatsWidget(widgetEl, config);
            break;
        default:
            console.warn('Unknown widget type:', type);
    }
}

// Uptime Kuma Widget
async function initUptimeKumaWidget(widgetEl, config) {
    const contentEl = widgetEl.querySelector('.widget-content');

    if (!config.url || !config.slug) {
        contentEl.innerHTML = '<p class="error">Widget not configured. Set url and slug in config.</p>';
        return;
    }

    // Add toggle button and status indicator to widget header
    const titleEl = widgetEl.querySelector('.widget-title');
    if (titleEl && !titleEl.parentElement.classList.contains('widget-header')) {
        const header = document.createElement('div');
        header.className = 'widget-header';
        titleEl.parentElement.insertBefore(header, titleEl);
        header.appendChild(titleEl);

        // Create overall status container (will be populated on render)
        const overallStatus = document.createElement('div');
        overallStatus.className = 'overall-status';
        header.appendChild(overallStatus);

        const toggleBtn = document.createElement('button');
        toggleBtn.className = 'widget-toggle';
        toggleBtn.innerHTML = '▼';
        toggleBtn.setAttribute('aria-label', 'Toggle widget');
        header.appendChild(toggleBtn);

        // Store collapsed state
        const isCollapsed = config.collapsed === 'true' || config.collapsed === true;
        widgetEl.dataset.collapsed = isCollapsed ? 'true' : 'false';

        // Set initial state
        if (isCollapsed) {
            widgetEl.classList.add('collapsed');
            toggleBtn.classList.add('collapsed');
        }

        // Toggle handler
        toggleBtn.addEventListener('click', () => {
            const collapsed = widgetEl.dataset.collapsed === 'true';
            widgetEl.dataset.collapsed = !collapsed ? 'true' : 'false';
            widgetEl.classList.toggle('collapsed');
            toggleBtn.classList.toggle('collapsed');
        });
    }

    try {
        const response = await fetch(`/api/widget/uptime-kuma?url=${encodeURIComponent(config.url)}&slug=${encodeURIComponent(config.slug)}`);

        if (!response.ok) {
            throw new Error('Failed to fetch status');
        }

        const data = await response.json();

        renderUptimeKumaWidget(widgetEl, contentEl, data);

        // Refresh every 60 seconds
        setInterval(() => {
            fetch(`/api/widget/uptime-kuma?url=${encodeURIComponent(config.url)}&slug=${encodeURIComponent(config.slug)}`)
                .then(r => r.json())
                .then(d => renderUptimeKumaWidget(widgetEl, contentEl, d))
                .catch(e => console.error('Widget refresh failed:', e));
        }, 60000);

    } catch (error) {
        contentEl.innerHTML = `
            <p class="error" style="color: #e74c3c;">
                Unable to connect to Uptime Kuma<br>
                <small style="color: rgba(255,255,255,0.5);">${config.url}</small>
            </p>
        `;
    }
}

function renderUptimeKumaWidget(widgetEl, contentEl, data) {
    if (!data || !data.publicGroupList) {
        contentEl.innerHTML = '<p>No status data available</p>';
        return;
    }

    // Calculate overall status and collect monitor info
    let overallStatus = 'up'; // Start optimistic
    let statusCounts = { up: 0, down: 0, pending: 0 };
    const monitors = [];

    data.publicGroupList.forEach(group => {
        group.monitorList.forEach(monitor => {
            // Get the latest heartbeat for this monitor from heartbeatList
            const heartbeats = data.heartbeatList?.[monitor.id] || [];
            const latestHeartbeat = heartbeats.length > 0 ? heartbeats[heartbeats.length - 1] : null;

            // Extract status from the latest heartbeat
            let statusValue = latestHeartbeat?.status;

            // Determine status
            let status;
            if (statusValue === 1 || statusValue === true) {
                status = 'up';
            } else if (statusValue === 0 || statusValue === false) {
                status = 'down';
            } else {
                status = 'pending';
            }

            statusCounts[status]++;

            // Get uptime percentage for this monitor
            const uptimeKey = `${monitor.id}_24`;
            const uptimeValue = data.uptimeList?.[uptimeKey];
            const uptime = uptimeValue !== undefined ? (uptimeValue * 100).toFixed(1) + '%' : '-';

            monitors.push({
                name: monitor.name,
                status: status,
                uptime: uptime
            });
        });
    });

    // Determine overall status: down takes priority, then pending, then up
    if (statusCounts.down > 0) {
        overallStatus = 'down';
    } else if (statusCounts.pending > 0) {
        overallStatus = 'pending';
    }

    // Update overall status in header
    const overallStatusEl = widgetEl.querySelector('.overall-status');
    if (overallStatusEl) {
        overallStatusEl.innerHTML = `
            <div class="overall-status-indicator status-${overallStatus}"></div>
            <div class="overall-status-text">
                ${statusCounts.up} up${statusCounts.down > 0 ? `, ${statusCounts.down} down` : ''}${statusCounts.pending > 0 ? `, ${statusCounts.pending} pending` : ''}
            </div>
        `;
    }

    // Build detailed list (always in contentEl)
    let html = '<ul class="status-list">';
    monitors.forEach(monitor => {
        const statusClass = `status-${monitor.status}`;
        html += `
            <li class="status-item">
                <span class="status-indicator ${statusClass}"></span>
                <span class="status-name">${escapeHtml(monitor.name)}</span>
                <span class="status-uptime">${monitor.uptime}</span>
            </li>
        `;
    });
    html += '</ul>';

    contentEl.innerHTML = html;
}

// Iframe Widget
function initIframeWidget(widgetEl, config) {
    const contentEl = widgetEl.querySelector('.widget-content');
    
    if (!config.url) {
        contentEl.innerHTML = '<p class="error">Widget not configured. Set url in config.</p>';
        return;
    }

    const height = config.height || '200px';
    contentEl.innerHTML = `
        <iframe 
            src="${escapeHtml(config.url)}" 
            style="width: 100%; height: ${height}; border: none; border-radius: 4px;"
            loading="lazy"
        ></iframe>
    `;
}

// Clock Widget
function initClockWidget(widgetEl, config) {
    const contentEl = widgetEl.querySelector('.widget-content');
    const timezone = config.timezone || 'local';
    const format = config.format || '24h';

    function updateClock() {
        const now = new Date();
        let options = {
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: format === '12h'
        };
        
        if (timezone !== 'local') {
            options.timeZone = timezone;
        }

        const timeStr = now.toLocaleTimeString('en-US', options);
        const dateStr = now.toLocaleDateString('en-US', {
            weekday: 'long',
            year: 'numeric',
            month: 'long',
            day: 'numeric',
            ...(timezone !== 'local' ? { timeZone: timezone } : {})
        });

        contentEl.innerHTML = `
            <div style="text-align: center;">
                <div style="font-size: 3rem; font-weight: 300; margin-bottom: 0.5rem;">${timeStr}</div>
                <div style="color: rgba(255,255,255,0.6); font-size: 1.2rem;">${dateStr}</div>
            </div>
        `;
    }

    updateClock();
    setInterval(updateClock, 1000);
}

// System Stats Widget
function initSystemStatsWidget(widgetEl, config) {
    const contentEl = widgetEl.querySelector('.widget-content');
    const refreshSeconds = parseInt(config.refresh || config.interval, 10);
    const refreshMs = Number.isFinite(refreshSeconds) && refreshSeconds > 0 ? refreshSeconds * 1000 : 5000;
    let intervalId = null;

    async function fetchAndRender() {
        try {
            const response = await fetch('/api/widget/system-stats');
            if (!response.ok) {
                if (response.status === 501) {
                    contentEl.innerHTML = `
                        <p class="error" style="color: rgba(255, 255, 255, 0.6);">
                            System stats are available on Linux only.
                        </p>
                    `;
                    if (intervalId) {
                        clearInterval(intervalId);
                    }
                    return;
                }
                throw new Error('Failed to fetch system stats');
            }

            const data = await response.json();
            renderSystemStatsWidget(contentEl, data);
        } catch (error) {
            contentEl.innerHTML = `
                <p class="error" style="color: #e74c3c;">
                    Unable to load system stats
                </p>
            `;
        }
    }

    fetchAndRender();
    intervalId = setInterval(fetchAndRender, refreshMs);
}

function renderSystemStatsWidget(contentEl, data) {
    const cpuPercent = Math.max(0, Math.min(100, data?.cpu?.usage_percent || 0));
    const memTotal = data?.memory?.total_bytes || 0;
    const memUsed = data?.memory?.used_bytes || 0;
    const memPercent = Math.max(0, Math.min(100, data?.memory?.used_percent || 0));

    const memUsedText = formatBytes(memUsed);
    const memTotalText = formatBytes(memTotal);

    contentEl.innerHTML = `
        <div class="system-stats">
            <div class="stat-row">
                <div class="stat-label">CPU</div>
                <div class="stat-value">${cpuPercent.toFixed(1)}%</div>
            </div>
            <div class="stat-bar">
                <div class="stat-fill stat-fill-cpu" style="width: ${cpuPercent}%"></div>
            </div>
            <div class="stat-row">
                <div class="stat-label">RAM</div>
                <div class="stat-value">${memUsedText} / ${memTotalText}</div>
            </div>
            <div class="stat-bar">
                <div class="stat-fill stat-fill-ram" style="width: ${memPercent}%"></div>
            </div>
            <div class="stat-sub">${memPercent.toFixed(1)}% used</div>
        </div>
    `;
}

function formatBytes(bytes) {
    if (!bytes || bytes <= 0) {
        return '0 B';
    }

    const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
    let size = bytes;
    let unitIndex = 0;
    while (size >= 1024 && unitIndex < units.length - 1) {
        size /= 1024;
        unitIndex++;
    }

    const precision = size >= 100 ? 0 : size >= 10 ? 1 : 2;
    return `${size.toFixed(precision)} ${units[unitIndex]}`;
}

// Utility function to escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
