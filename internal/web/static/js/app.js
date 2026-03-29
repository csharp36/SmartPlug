// SmartPlug Web App JavaScript

// API helper functions
async function apiGet(endpoint) {
    const response = await fetch(endpoint);
    if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
    }
    return response.json();
}

async function apiPost(endpoint, data = {}) {
    const response = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error || `API error: ${response.status}`);
    }
    return response.json();
}

// Dashboard functionality
function initDashboard() {
    // Set up auto-refresh
    setInterval(refreshStatus, 5000);

    // Set up button handlers
    const heatNowBtn = document.getElementById('btn-heat-now');
    const stopBtn = document.getElementById('btn-stop');
    const toggleBtn = document.getElementById('btn-toggle-enable');

    if (heatNowBtn) {
        heatNowBtn.addEventListener('click', handleHeatNow);
    }

    if (stopBtn) {
        stopBtn.addEventListener('click', handleStop);
    }

    if (toggleBtn) {
        toggleBtn.addEventListener('click', handleToggleEnable);
    }
}

async function refreshStatus() {
    try {
        const status = await apiGet('/api/status');
        updateDashboard(status);
    } catch (err) {
        console.error('Failed to refresh status:', err);
    }
}

function updateDashboard(status) {
    // Update pump state
    const pumpStateEl = document.getElementById('pump-state');
    const pumpTriggerEl = document.getElementById('pump-trigger');
    const pumpCard = document.querySelector('.pump-card');

    if (pumpStateEl) {
        pumpStateEl.textContent = status.pump.state;
    }

    if (pumpTriggerEl) {
        pumpTriggerEl.textContent = status.pump.is_running
            ? `Trigger: ${status.pump.last_trigger}`
            : 'Idle';
    }

    if (pumpCard) {
        pumpCard.classList.toggle('active', status.pump.is_running);
        const indicator = pumpCard.querySelector('.card-indicator');
        if (indicator) {
            indicator.classList.toggle('on', status.pump.is_running);
            indicator.classList.toggle('off', !status.pump.is_running);
        }
    }

    // Update temperatures
    const tempHotEl = document.getElementById('temp-hot');
    const tempReturnEl = document.getElementById('temp-return');
    const tempDiffEl = document.getElementById('temp-diff');

    if (tempHotEl && status.temperatures.hot_valid) {
        tempHotEl.textContent = `${status.temperatures.hot_outlet.toFixed(1)}°F`;
    }

    if (tempReturnEl && status.temperatures.return_valid) {
        tempReturnEl.textContent = `${status.temperatures.return_line.toFixed(1)}°F`;
    }

    if (tempDiffEl && status.temperatures.hot_valid && status.temperatures.return_valid) {
        tempDiffEl.textContent = `${status.temperatures.differential.toFixed(1)}°F`;
    }

    // Update flow state
    const flowStateEl = document.getElementById('flow-state');
    const flowCard = document.querySelector('.flow-card');

    if (flowStateEl && status.flow) {
        flowStateEl.textContent = status.flow.active ? 'Active' : 'Idle';
    }

    if (flowCard && status.flow) {
        flowCard.classList.toggle('active', status.flow.active);
        const indicator = flowCard.querySelector('.card-indicator');
        if (indicator) {
            indicator.classList.toggle('on', status.flow.active);
            indicator.classList.toggle('off', !status.flow.active);
        }
    }

    // Update schedule state
    const scheduleStateEl = document.getElementById('schedule-state');
    if (scheduleStateEl) {
        if (!status.schedule.enabled) {
            scheduleStateEl.textContent = 'Disabled';
        } else if (status.schedule.in_window) {
            scheduleStateEl.textContent = 'Active';
        } else {
            scheduleStateEl.textContent = 'Enabled';
        }
    }

    // Update buttons
    const heatNowBtn = document.getElementById('btn-heat-now');
    const stopBtn = document.getElementById('btn-stop');
    const toggleBtn = document.getElementById('btn-toggle-enable');

    if (heatNowBtn) {
        heatNowBtn.disabled = status.pump.is_running;
    }

    if (stopBtn) {
        stopBtn.disabled = !status.pump.is_running;
    }

    if (toggleBtn) {
        toggleBtn.classList.toggle('btn-success', status.pump.enabled);
        toggleBtn.classList.toggle('btn-danger', !status.pump.enabled);
        toggleBtn.textContent = status.pump.enabled ? 'Enabled' : 'Disabled';
    }

    // Update info section
    const controllerStateEl = document.getElementById('controller-state');
    const lastTriggerEl = document.getElementById('last-trigger');
    const runtimeEl = document.getElementById('runtime');

    if (controllerStateEl) {
        controllerStateEl.textContent = status.pump.state;
    }

    if (lastTriggerEl) {
        lastTriggerEl.textContent = status.pump.last_trigger;
    }

    if (runtimeEl && status.pump.runtime_seconds > 0) {
        runtimeEl.textContent = `${Math.round(status.pump.runtime_seconds)}s`;
    }
}

async function handleHeatNow() {
    const btn = document.getElementById('btn-heat-now');
    btn.disabled = true;

    try {
        await apiPost('/api/pump/heat-now');
        await refreshStatus();
    } catch (err) {
        alert(`Failed to activate pump: ${err.message}`);
    } finally {
        btn.disabled = false;
    }
}

async function handleStop() {
    const btn = document.getElementById('btn-stop');
    btn.disabled = true;

    try {
        await apiPost('/api/pump/stop');
        await refreshStatus();
    } catch (err) {
        alert(`Failed to stop pump: ${err.message}`);
    } finally {
        btn.disabled = false;
    }
}

async function handleToggleEnable() {
    const btn = document.getElementById('btn-toggle-enable');
    const isEnabled = btn.classList.contains('btn-success');

    try {
        if (isEnabled) {
            await apiPost('/api/pump/disable');
        } else {
            await apiPost('/api/pump/enable');
        }
        await refreshStatus();
    } catch (err) {
        alert(`Failed to toggle pump: ${err.message}`);
    }
}

// Day name helper
const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

function dayName(dayNum) {
    return dayNames[dayNum] || '';
}

// Format time for display
function formatTime(timeStr) {
    if (!timeStr) return '';
    const [hours, minutes] = timeStr.split(':');
    const h = parseInt(hours, 10);
    const suffix = h >= 12 ? 'PM' : 'AM';
    const h12 = h % 12 || 12;
    return `${h12}:${minutes} ${suffix}`;
}

// Service worker registration for PWA
if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
        navigator.serviceWorker.register('/static/sw.js').catch(err => {
            console.log('ServiceWorker registration failed:', err);
        });
    });
}

// Export functions for use in templates
window.initDashboard = initDashboard;
window.dayName = dayName;
window.formatTime = formatTime;
