// Load Servers List
async function loadServers() {
    try {
        const res = await apiFetch('/api/servers');
        const data = await res.json();
        const tbody = document.querySelector('#servers-list-table tbody');
        tbody.innerHTML = '';

        if (data.length === 0) {
            tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--muted);">No servers connected. Use the form above to connect a server.</td></tr>`;
            return;
        }

        data.forEach(s => {
            let statusClass = 'status-unknown';
            const statusStr = (s.status || '').toLowerCase();
            if (statusStr === 'online') statusClass = 'status-online';
            else if (statusStr === 'connecting') statusClass = 'status-connecting';
            else if (statusStr === 'offline') statusClass = 'status-offline';

            let lastUpdatedDisplay = 'Never';
            if (s.last_updated && s.last_updated > 0) {
                const d = new Date(s.last_updated * 1000);
                lastUpdatedDisplay = d.toLocaleString();
            }

            tbody.innerHTML += `
                <tr>
                    <td>
                        <div style="display: flex; align-items: center; gap: 12px;">
                            <div class="server-chip" onclick="enterServerView(${s.id}, '${s.name}')">≡</div>
                            <span style="font-weight: bold; cursor: pointer;" onclick="enterServerView(${s.id}, '${s.name}')">${s.name}</span>
                        </div>
                    </td>
                    <td>${s.domain || 'Unknown'}</td>
                    <td>
                        <span class="status-dot ${statusClass}"></span>
                        <span style="font-weight: bold;">${s.status}</span>
                    </td>
                    <td>${lastUpdatedDisplay}</td>
                    <td style="text-align: right;">
                        <button class="btn btn-secondary btn-icon" onclick="refreshServer(${s.id})">🔄</button>
                        <button class="btn btn-secondary btn-icon btn-danger" onclick="deleteServer(${s.id}, '${s.name}')">🗑️</button>
                    </td>
                </tr>
            `;
        });
    } catch (e) {
        console.error(e);
    }
}

async function addServer() {
    const name = document.getElementById('srv-name').value;
    const address = document.getElementById('srv-address').value;
    const port = document.getElementById('srv-port').value;
    if (!name || !address || !port) {
        alert("Please fill all server connection fields.");
        return;
    }
    try {
        const res = await apiFetch('/api/servers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, address, port })
        });
        if (res.ok) {
            document.getElementById('srv-name').value = '';
            document.getElementById('srv-address').value = '';
            document.getElementById('srv-port').value = '';
            loadServers();
        } else {
            const err = await res.json();
            alert("Error connecting: " + (err.error || 'unknown'));
        }
    } catch (e) {
        alert("Failed to send request.");
    }
}

async function refreshServer(id) {
    await apiFetch(`/api/servers/${id}/refresh`, { method: 'POST' });
    loadServers();
}

async function deleteServer(id, name) {
    if (!confirm(`Are you sure you want to remove server "${name}"?`)) return;
    await apiFetch(`/api/servers/${id}`, { method: 'DELETE' });
    loadServers();
}
