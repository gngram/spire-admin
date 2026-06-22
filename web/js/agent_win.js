// Registered Agents Actions
async function loadServerAgents() {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/agents`);
        const data = await res.json();
        const container = document.getElementById('agents-container');
        container.innerHTML = '';
        if (!data || data.length === 0) {
            container.innerHTML = `<div style="text-align: center; color: var(--muted); padding: 20px;">No agents registered.</div>`;
            return;
        }
        data.forEach(a => {
            const bannedClass = a.Banned ? 'banned' : '';
            container.innerHTML += `
                <div class="agent-box ${bannedClass}">
                    <div class="agent-info">
                        <div class="agent-spiffeid">${a.SPIFFEID}</div>
                    </div>
                    <div class="agent-actions">
                        <button class="btn btn-secondary" onclick="showAgentInfo('${a.SPIFFEID}')">Info</button>
                        <button class="btn btn-secondary" onclick="evictAgent('${a.SPIFFEID}')">Evict</button>
                        <button class="btn btn-secondary btn-danger" onclick="banAgent('${a.SPIFFEID}')">Ban</button>
                    </div>
                </div>
            `;
        });
    } catch (e) {
        alert("Failed to load agents.");
    }
}

async function showAgentInfo(spiffeID) {
    try {

        const res = await apiFetch(`/api/servers/${activeServerId}/agents/info?spiffe_id=${encodeURIComponent(spiffeID)}`);
        const data = await res.json();
        document.getElementById('info-modal-title').innerText = "Agent Details";
        if (data.info && typeof data.info === 'object') {
            const cleanText = Object.entries(data.info)
                .map(([key, value]) => {
                    const displayValue = typeof value === 'object' ? JSON.stringify(value) : value;
                    return `${key}: ${displayValue}`;
                })
                .join('\n');

            document.getElementById('info-modal-text').innerText = cleanText;
        } else {
            document.getElementById('info-modal-text').innerText = data.info || "No details.";
        }
        openModal('info-modal');
    } catch (e) {
        alert("Failed to get agent info.");
    }
}

async function evictAgent(spiffeID) {
    if (!confirm(`Are you sure you want to evict agent ${spiffeID}?`)) return;
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/agents/evict`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ spiffe_id: spiffeID })
        });
        if (res.ok) {
            loadServerAgents();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}

async function banAgent(spiffeID) {
    if (!confirm(`Are you sure you want to ban agent ${spiffeID}?`)) return;
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/agents/ban`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ spiffe_id: spiffeID })
        });
        if (res.ok) {
            loadServerAgents();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}

async function purgeExpiredAgents() {
    if (!confirm(`Are you sure you want to purge all expired agents?`)) return;
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/agents/purge-expired`, { method: 'POST' });
        if (res.ok) {
            loadServerAgents();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}
