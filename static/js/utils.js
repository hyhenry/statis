// Shared utility functions

/**
 * Escape HTML special characters to prevent XSS
 * @param {string} text - The text to escape
 * @returns {string} - The escaped HTML string
 */
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

/**
 * Format a byte count as a human-readable string (B, KB, MB, GB, TB)
 * @param {number} bytes
 * @param {number} [decimals=1]
 * @returns {string}
 */
function formatBytes(bytes, decimals = 1) {
    if (!bytes || bytes < 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
    const value = bytes / Math.pow(1024, i);
    return `${value.toFixed(i === 0 ? 0 : decimals)} ${units[i]}`;
}

/**
 * Format a duration in seconds as "Xd Yh Zm"
 * @param {number} seconds
 * @returns {string}
 */
function formatUptime(seconds) {
    if (!seconds || seconds < 0) return '-';
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    if (d > 0) return `${d}d ${h}h ${m}m`;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
}
