// Widget initialization and handling

document.addEventListener('DOMContentLoaded', function() {
    // Initialize all widgets
    const widgets = document.querySelectorAll('.widget');
    widgets.forEach(initWidget);
});

// Helper: Render widget error message using CSS classes from base.css
function renderWidgetError(message, detail = '') {
    return `<p class="widget-error">${escapeHtml(message)}${detail ? `<br><small class="widget-error-url">${escapeHtml(detail)}</small>` : ''}</p>`;
}

// Helper: Render muted info message
function renderWidgetInfo(message) {
    return `<p class="widget-error-muted">${escapeHtml(message)}</p>`;
}

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
        case 'rss':
            initRSSWidget(widgetEl, config);
            break;
        case 'header':
            initHeaderWidget(widgetEl, config);
            break;
        case 'truenas-scale':
            initTrueNASWidget(widgetEl, config);
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
        contentEl.innerHTML = renderWidgetError('Unable to connect to Uptime Kuma', config.url);
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
                    contentEl.innerHTML = renderWidgetInfo('System stats are available on Linux only.');
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
            contentEl.innerHTML = renderWidgetError('Unable to load system stats');
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

// RSS Widget
async function initRSSWidget(widgetEl, config) {
    const contentEl = widgetEl.querySelector('.widget-content');
    const titleEl = widgetEl.querySelector('.widget-title');

    if (titleEl && !titleEl.parentElement.classList.contains('widget-header')) {
        const header = document.createElement('div');
        header.className = 'widget-header';
        titleEl.parentElement.insertBefore(header, titleEl);
        header.appendChild(titleEl);
    }

    if (!config.url) {
        contentEl.innerHTML = '<p class="error">Widget not configured. Set url in config.</p>';
        return;
    }

    const itemsPerPage = parseInt(config.items_per_page) || 3;
    const refreshSeconds = parseInt(config.refresh) || 300;
    let currentPage = 0;
    let allItems = [];

    async function fetchFeed() {
        try {
            const response = await fetch(`/api/widget/rss?url=${encodeURIComponent(config.url)}`);
            if (!response.ok) {
                throw new Error('Failed to fetch RSS feed');
            }
            const data = await response.json();
            allItems = data.items || [];
            currentPage = 0;
            renderRSSWidget();
        } catch (error) {
            contentEl.innerHTML = renderWidgetError('Unable to load RSS feed', config.url);
        }
    }

    function renderRSSWidget() {
        if (allItems.length === 0) {
            contentEl.innerHTML = '<p class="rss-empty">No articles found</p>';
            return;
        }

        const totalPages = Math.ceil(allItems.length / itemsPerPage);
        const startIdx = currentPage * itemsPerPage;
        const pageItems = allItems.slice(startIdx, startIdx + itemsPerPage);

        let html = '<div class="rss-items">';
        pageItems.forEach(item => {
            const date = formatRSSDate(item.pub_date);
            const description = truncateText(item.description, 120);
            html += `
                <div class="rss-item">
                    ${item.image ? `
                        <div class="rss-thumb">
                            <img src="${escapeHtml(item.image)}" alt="" loading="lazy" onerror="this.parentElement.style.display='none'">
                        </div>
                    ` : ''}
                    <div class="rss-body">
                        <a href="${escapeHtml(item.link)}" target="_blank" class="rss-title">${escapeHtml(item.title)}</a>
                        <p class="rss-description">${escapeHtml(description)}</p>
                        ${date ? `<span class="rss-date">${date}</span>` : ''}
                    </div>
                </div>
            `;
        });
        html += '</div>';

        // Pagination controls
        if (totalPages > 1) {
            html += `
                <div class="rss-pagination">
                    <button class="rss-nav-btn rss-prev" ${currentPage === 0 ? 'disabled' : ''}>&#9664;</button>
                    <span class="rss-page-info">${currentPage + 1} / ${totalPages}</span>
                    <button class="rss-nav-btn rss-next" ${currentPage >= totalPages - 1 ? 'disabled' : ''}>&#9654;</button>
                </div>
            `;
        }

        contentEl.innerHTML = html;

        // Attach event listeners for pagination
        const prevBtn = contentEl.querySelector('.rss-prev');
        const nextBtn = contentEl.querySelector('.rss-next');

        if (prevBtn) {
            prevBtn.addEventListener('click', () => {
                if (currentPage > 0) {
                    currentPage--;
                    renderRSSWidget();
                }
            });
        }

        if (nextBtn) {
            nextBtn.addEventListener('click', () => {
                if (currentPage < totalPages - 1) {
                    currentPage++;
                    renderRSSWidget();
                }
            });
        }
    }

    await fetchFeed();
    setInterval(fetchFeed, refreshSeconds * 1000);
}

function initHeaderWidget(widgetEl, config) {
    const titleEl = widgetEl.querySelector('.widget-title');
    const title = titleEl ? titleEl.textContent.trim() : (config?.title || 'Section');
    widgetEl.innerHTML = `<h4 class="section-title">${escapeHtml(title)}</h4>`;
}

function formatRSSDate(dateStr) {
    if (!dateStr) return '';
    try {
        const date = new Date(dateStr);
        if (isNaN(date.getTime())) return '';
        return date.toLocaleDateString('en-US', {
            month: 'short',
            day: 'numeric',
            year: 'numeric'
        });
    } catch {
        return '';
    }
}

function truncateText(text, maxLength) {
    if (!text) return '';
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength).trim() + '...';
}

// escapeHtml is provided by utils.js

// TrueNAS SCALE Widget
function initTrueNASWidget(widgetEl, config) {
    const contentEl = widgetEl.querySelector('.widget-content');

    if (!config.url || !config.api_key) {
        contentEl.innerHTML = renderWidgetError('Widget not configured. Set url and api_key in config.');
        return;
    }

    ensureCollapsibleHeader(widgetEl, config);

    const params = new URLSearchParams({ url: config.url, api_key: config.api_key });
    if (config.show_system !== undefined) params.set('show_system', String(config.show_system));
    if (config.show_pools !== undefined) params.set('show_pools', String(config.show_pools));
    if (config.show_disks !== undefined) params.set('show_disks', String(config.show_disks));
    if (config.show_backups !== undefined) params.set('show_backups', String(config.show_backups));
    const endpoint = `/api/widget/truenas-scale?${params.toString()}`;

    async function refresh() {
        try {
            const response = await fetch(endpoint);
            if (!response.ok) throw new Error(`HTTP ${response.status}`);
            const data = await response.json();
            renderTrueNASWidget(widgetEl, contentEl, data);
        } catch (err) {
            contentEl.innerHTML = renderWidgetError('Unable to connect to TrueNAS', config.url);
        }
    }

    refresh();
    setInterval(refresh, 60000);
}

function ensureCollapsibleHeader(widgetEl, config) {
    const titleEl = widgetEl.querySelector('.widget-title');
    if (!titleEl || titleEl.parentElement.classList.contains('widget-header')) return;

    const header = document.createElement('div');
    header.className = 'widget-header';
    titleEl.parentElement.insertBefore(header, titleEl);
    header.appendChild(titleEl);

    const overallStatus = document.createElement('div');
    overallStatus.className = 'overall-status';
    header.appendChild(overallStatus);

    const toggleBtn = document.createElement('button');
    toggleBtn.className = 'widget-toggle';
    toggleBtn.innerHTML = '▼';
    toggleBtn.setAttribute('aria-label', 'Toggle widget');
    header.appendChild(toggleBtn);

    const isCollapsed = config.collapsed === 'true' || config.collapsed === true;
    widgetEl.dataset.collapsed = isCollapsed ? 'true' : 'false';
    if (isCollapsed) {
        widgetEl.classList.add('collapsed');
        toggleBtn.classList.add('collapsed');
    }

    toggleBtn.addEventListener('click', () => {
        const collapsed = widgetEl.dataset.collapsed === 'true';
        widgetEl.dataset.collapsed = !collapsed ? 'true' : 'false';
        widgetEl.classList.toggle('collapsed');
        toggleBtn.classList.toggle('collapsed');
    });
}

function renderTrueNASWidget(widgetEl, contentEl, data) {
    const parts = [];
    let overall = 'up';

    if (data.system) {
        parts.push(renderTrueNASSystem(data.system));
    } else if (data.system_error) {
        parts.push(renderTrueNASSectionError('System', data.system_error));
    }

    if (data.pools) {
        const { html, status } = renderTrueNASPools(data.pools);
        parts.push(html);
        if (status === 'down') overall = 'down';
    } else if (data.pools_error) {
        parts.push(renderTrueNASSectionError('Pools', data.pools_error));
        overall = 'down';
    }

    if (data.disks) {
        parts.push(renderTrueNASDisks(data.disks));
    } else if (data.disks_error) {
        parts.push(renderTrueNASSectionError('Disks', data.disks_error));
    }

    if (data.backups) {
        const { html, status } = renderTrueNASBackups(data.backups);
        parts.push(html);
        if (status === 'down') overall = 'down';
    } else if (data.backups_error) {
        parts.push(renderTrueNASSectionError('Backups', data.backups_error));
    }

    const overallStatusEl = widgetEl.querySelector('.overall-status');
    if (overallStatusEl) {
        const hostname = data.system?.hostname || '';
        overallStatusEl.innerHTML = `
            <div class="overall-status-indicator status-${overall}"></div>
            <div class="overall-status-text">${escapeHtml(hostname)}</div>
        `;
    }

    contentEl.innerHTML = parts.length ? parts.join('') : renderWidgetInfo('No sections enabled.');
}

function renderTrueNASSystem(sys) {
    return `
        <div class="truenas-section">
            <h5 class="truenas-section-title">System</h5>
            <dl class="truenas-kv">
                <dt>Hostname</dt><dd>${escapeHtml(sys.hostname || '-')}</dd>
                <dt>Version</dt><dd>${escapeHtml(sys.version || '-')}</dd>
                <dt>Uptime</dt><dd>${escapeHtml(formatUptime(sys.uptime_seconds))}</dd>
                <dt>CPU</dt><dd>${escapeHtml(sys.cpu_model || '-')}${sys.cpu_cores ? ` (${sys.cpu_cores} cores)` : ''}</dd>
                <dt>Memory</dt><dd>${escapeHtml(formatBytes(sys.memory_bytes))}</dd>
            </dl>
        </div>
    `;
}

function renderTrueNASPools(pools) {
    let status = 'up';
    const rows = pools.map(p => {
        const vdevErrors = (p.read_errors || 0) + (p.write_errors || 0) + (p.checksum_errors || 0);
        const hasErrors = vdevErrors > 0 || (p.scan_errors || 0) > 0;
        const healthy = p.healthy && (p.status === '' || /online/i.test(p.status)) && !hasErrors;
        if (!healthy) status = 'down';
        const pct = Math.max(0, Math.min(100, p.used_percent || 0));

        const warnings = [];
        if (p.status_detail) {
            warnings.push(`<div class="truenas-pool-warn">${escapeHtml(p.status_detail)}</div>`);
        }
        if (vdevErrors > 0) {
            warnings.push(`<div class="truenas-pool-warn">ZFS errors: ${p.read_errors} read, ${p.write_errors} write, ${p.checksum_errors} checksum</div>`);
        }
        if ((p.scan_errors || 0) > 0) {
            warnings.push(`<div class="truenas-pool-warn">Last ${(p.scan_function || 'scan').toLowerCase()}: ${p.scan_errors} errors</div>`);
        }
        const scanInfo = renderScanInfo(p);

        return `
            <div class="truenas-pool-row">
                <div class="truenas-pool-head">
                    <span class="status-indicator ${healthy ? 'status-up' : 'status-down'}"></span>
                    <span class="truenas-pool-name">${escapeHtml(p.name)}</span>
                    <span class="truenas-pool-status">${escapeHtml(p.status || (healthy ? 'ONLINE' : 'UNKNOWN'))}</span>
                </div>
                <div class="stat-bar"><div class="stat-fill stat-fill-ram" style="width: ${pct.toFixed(1)}%"></div></div>
                <div class="truenas-pool-meta">${escapeHtml(formatBytes(p.allocated_bytes))} / ${escapeHtml(formatBytes(p.size_bytes))} (${pct.toFixed(1)}%)</div>
                ${warnings.join('')}
                ${scanInfo}
            </div>
        `;
    }).join('');
    const html = `
        <div class="truenas-section">
            <h5 class="truenas-section-title">Pools</h5>
            ${pools.length ? rows : renderWidgetInfo('No pools found.')}
        </div>
    `;
    return { html, status };
}

function renderScanInfo(p) {
    if (!p.scan_state && !p.scan_function) return '';
    const fn = (p.scan_function || 'scan').toLowerCase();
    const state = (p.scan_state || '').toLowerCase();

    if (state === 'scanning') {
        const pct = (p.scan_percent || 0).toFixed(1);
        return `<div class="truenas-pool-scan">${escapeHtml(fn)} in progress — ${pct}%</div>`;
    }
    if (p.scan_end_time) {
        const date = new Date(p.scan_end_time * 1000);
        const rel = formatRelativeTime(date);
        return `<div class="truenas-pool-scan">Last ${escapeHtml(fn)}: ${escapeHtml(rel)}</div>`;
    }
    return '';
}

function formatRelativeTime(date) {
    const diffMs = Date.now() - date.getTime();
    const days = Math.floor(diffMs / 86400000);
    if (days < 1) return 'today';
    if (days === 1) return 'yesterday';
    if (days < 30) return `${days} days ago`;
    const months = Math.floor(days / 30);
    if (months < 12) return `${months} month${months > 1 ? 's' : ''} ago`;
    return date.toLocaleDateString();
}

function renderTrueNASDisks(disks) {
    const rows = disks.map(d => {
        const tempClass = d.temperature >= 55 ? 'truenas-temp-danger'
            : d.temperature >= 45 ? 'truenas-temp-warn'
            : '';
        const tempStr = d.temperature ? `${d.temperature}°C` : '-';
        return `
            <div class="truenas-disk-row">
                <span class="truenas-disk-name">${escapeHtml(d.name)}</span>
                <span class="truenas-disk-model">${escapeHtml(d.model || '')}</span>
                <span class="truenas-disk-size">${escapeHtml(formatBytes(d.size_bytes))}</span>
                <span class="truenas-disk-temp ${tempClass}">${escapeHtml(tempStr)}</span>
            </div>
        `;
    }).join('');
    return `
        <div class="truenas-section">
            <h5 class="truenas-section-title">Disks (${disks.length})</h5>
            <div class="truenas-disk-list">
                ${disks.length ? rows : renderWidgetInfo('No disks found.')}
            </div>
        </div>
    `;
}

function renderTrueNASBackups(backups) {
    let status = 'up';
    const rows = backups.map(b => {
        const state = (b.state || 'NEVER').toUpperCase();
        let dot = 'status-pending';
        let stateLabel = state;
        if (state === 'SUCCESS') dot = 'status-up';
        else if (state === 'FAILED') { dot = 'status-down'; status = 'down'; }
        else if (state === 'RUNNING') dot = 'status-pending';
        else if (state === 'NEVER') { dot = 'status-pending'; stateLabel = 'never run'; }

        const kindBadge = b.kind === 'cloudsync' ? 'Cloud' : 'Rsync';
        const dir = b.direction ? ` · ${escapeHtml(b.direction.toLowerCase())}` : '';
        const disabled = b.enabled ? '' : ' <span class="truenas-backup-disabled">(disabled)</span>';

        let subline = '';
        if (state === 'RUNNING') {
            subline = `running — ${(b.progress_percent || 0).toFixed(0)}%`;
        } else if (b.last_run_unix) {
            const rel = formatRelativeTime(new Date(b.last_run_unix * 1000));
            subline = `${stateLabel.toLowerCase()} · ${escapeHtml(rel)}`;
        } else {
            subline = stateLabel.toLowerCase();
        }

        const errLine = (state === 'FAILED' && b.error)
            ? `<div class="truenas-backup-error" title="${escapeHtml(b.error)}">${escapeHtml(truncateText(b.error, 120))}</div>`
            : '';

        return `
            <div class="truenas-backup-row">
                <span class="status-indicator ${dot}"></span>
                <div class="truenas-backup-main">
                    <div class="truenas-backup-title">
                        <span class="truenas-backup-kind">${kindBadge}</span>
                        <span class="truenas-backup-name">${escapeHtml(b.description || '(unnamed)')}${disabled}</span>
                    </div>
                    <div class="truenas-backup-sub">${subline}${dir}</div>
                    ${errLine}
                </div>
            </div>
        `;
    }).join('');

    const html = `
        <div class="truenas-section">
            <h5 class="truenas-section-title">Backups (${backups.length})</h5>
            ${backups.length ? rows : renderWidgetInfo('No backup tasks configured.')}
        </div>
    `;
    return { html, status };
}

function renderTrueNASSectionError(section, message) {
    return `
        <div class="truenas-section">
            <h5 class="truenas-section-title">${escapeHtml(section)}</h5>
            ${renderWidgetError(`Failed to load ${section.toLowerCase()}`, message)}
        </div>
    `;
}
