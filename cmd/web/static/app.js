// Network Monitor Web UI

let latencyChart = null;
let speedChart = null;
const maxDataPoints = 60;

// Initialize charts
function initCharts() {
    const latencyCtx = document.getElementById('latencyChart').getContext('2d');
    const speedCtx = document.getElementById('speedChart').getContext('2d');

    const commonOptions = {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
            x: {
                grid: { color: '#30363d' },
                ticks: { color: '#8b949e' }
            },
            y: {
                grid: { color: '#30363d' },
                ticks: { color: '#8b949e' }
            }
        },
        plugins: {
            legend: {
                labels: { color: '#8b949e' }
            }
        }
    };

    latencyChart = new Chart(latencyCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'Latency (ms)',
                data: [],
                borderColor: '#58a6ff',
                backgroundColor: 'rgba(88, 166, 255, 0.1)',
                tension: 0.4,
                fill: true
            }]
        },
        options: {
            ...commonOptions,
            scales: {
                ...commonOptions.scales,
                y: {
                    ...commonOptions.scales.y,
                    title: { display: true, text: 'ms', color: '#8b949e' }
                }
            }
        }
    });

    speedChart = new Chart(speedCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [
                {
                    label: 'Download (Mbps)',
                    data: [],
                    borderColor: '#39d2c0',
                    backgroundColor: 'rgba(57, 210, 192, 0.1)',
                    tension: 0.4,
                    fill: true
                },
                {
                    label: 'Upload (Mbps)',
                    data: [],
                    borderColor: '#3fb950',
                    backgroundColor: 'rgba(63, 185, 80, 0.1)',
                    tension: 0.4,
                    fill: true
                }
            ]
        },
        options: {
            ...commonOptions,
            scales: {
                ...commonOptions.scales,
                y: {
                    ...commonOptions.scales.y,
                    title: { display: true, text: 'Mbps', color: '#8b949e' }
                }
            }
        }
    });
}

// Update metrics grid
function updateMetricsGrid(metrics) {
    const grid = document.querySelector('.metrics-grid');
    if (!grid) return;

    const latencyColor = metrics.latency > 500 ? 'status-red' : 
                         metrics.latency > 200 ? 'status-yellow' : 'status-green';
    const lossColor = metrics.loss > 5 ? 'status-red' : 
                      metrics.loss > 0 ? 'status-yellow' : 'status-green';
    const barColor = metrics.latency > 500 ? 'bar-red' : 
                     metrics.latency > 200 ? 'bar-yellow' : 'bar-green';
    const lossBarColor = metrics.loss > 5 ? 'bar-red' : 'bar-green';

    grid.innerHTML = `
        <div class="metric-card">
            <div class="label">Latency</div>
            <div class="value ${latencyColor}">${metrics.latency.toFixed(0)}ms</div>
            <div class="bar"><div class="bar-fill ${barColor}" style="width: ${Math.min(metrics.latency / 5, 100)}%"></div></div>
        </div>
        <div class="metric-card">
            <div class="label">Jitter</div>
            <div class="value status-yellow">${metrics.jitter.toFixed(1)}ms</div>
        </div>
        <div class="metric-card">
            <div class="label">Packet Loss</div>
            <div class="value ${lossColor}">${metrics.loss.toFixed(1)}%</div>
            <div class="bar"><div class="bar-fill ${lossBarColor}" style="width: ${metrics.loss}%"></div></div>
        </div>
        <div class="metric-card">
            <div class="label">TCP (443)</div>
            <div class="value status-cyan">${metrics.tcp.toFixed(0)}ms</div>
        </div>
        <div class="metric-card">
            <div class="label">DNS</div>
            <div class="value status-yellow">${metrics.dns.toFixed(0)}ms</div>
        </div>
        <div class="metric-card">
            <div class="label">HTTP TTFB</div>
            <div class="value status-accent">${metrics.http.toFixed(0)}ms</div>
        </div>
        <div class="metric-card">
            <div class="label">Download</div>
            <div class="value status-cyan">${metrics.download_speed.toFixed(2)} Mbps</div>
        </div>
        <div class="metric-card">
            <div class="label">Upload</div>
            <div class="value status-green">${metrics.upload_speed.toFixed(2)} Mbps</div>
        </div>
    `;
}

// Update charts
function updateCharts(metrics) {
    const now = new Date().toLocaleTimeString();
    
    // Latency chart
    latencyChart.data.labels.push(now);
    latencyChart.data.datasets[0].data.push(metrics.latency);
    if (latencyChart.data.labels.length > maxDataPoints) {
        latencyChart.data.labels.shift();
        latencyChart.data.datasets[0].data.shift();
    }
    latencyChart.update();

    // Speed chart
    if (metrics.download_speed > 0 || metrics.upload_speed > 0) {
        speedChart.data.labels.push(now);
        speedChart.data.datasets[0].data.push(metrics.download_speed);
        speedChart.data.datasets[1].data.push(metrics.upload_speed);
        if (speedChart.data.labels.length > maxDataPoints) {
            speedChart.data.labels.shift();
            speedChart.data.datasets[0].data.shift();
            speedChart.data.datasets[1].data.shift();
        }
        speedChart.update();
    }
}

// Add log entry
function addLogEntry(message) {
    const logContent = document.getElementById('log-content');
    if (!logContent) return;
    
    const p = document.createElement('p');
    p.textContent = `[${new Date().toLocaleTimeString()}] ${message}`;
    logContent.insertBefore(p, logContent.firstChild);
    
    // Keep only last 50 entries
    while (logContent.children.length > 50) {
        logContent.removeChild(logContent.lastChild);
    }
}

// Handle HTMX response swap
document.body.addEventListener('htmx:afterOnLoad', function(evt) {
    if (evt.detail.pathInfo?.requestPath === '/api/metrics') {
        const metrics = evt.detail.xhr.responseText;
        try {
            const data = JSON.parse(metrics);
            updateMetricsGrid(data);
            updateCharts(data);
            
            if (data.issue_detected) {
                addLogEntry(`[!] ${data.issue_summary}`);
            }
        } catch (e) {
            console.error('Failed to parse metrics:', e);
        }
    }
});

// Pause button text update
document.body.addEventListener('htmx:afterSwap', function(evt) {
    if (evt.detail.target.id === 'pause-btn') {
        // Button text is already updated by HTMX via hx-swap
    }
});

// Initialize
document.addEventListener('DOMContentLoaded', function() {
    initCharts();
    
    // Add initial log entry
    addLogEntry('Dashboard loaded. Monitoring started.');
});
