// Federations Actions
let activeFederationTab = 'internal';

async function loadServerFederations() {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/federations`);
        const data = await res.json();
        const tbody = document.querySelector('#federations-table tbody');
        tbody.innerHTML = '';
        if (!data || data.length === 0) {
            tbody.innerHTML = `<tr><td colspan="3" style="text-align: center; color: var(--muted);">No federations configured.</td></tr>`;
            return;
        }
        data.forEach(f => {
            tbody.innerHTML += `
                <tr>
                    <td style="font-weight: bold;">${f.TrustDomain}</td>
                    <td>${f.Address}</td>
                    <td style="text-align: right;">
                        <button class="btn btn-secondary" onclick="showFederationInfo('${f.TrustDomain}')">Info</button>
                        <button class="btn btn-secondary" onclick="openFederationModal('${f.TrustDomain}')">Update</button>
                        <button class="btn btn-secondary" onclick="refreshFederation('${f.TrustDomain}')">Refresh</button>
                        <button class="btn btn-secondary btn-danger" onclick="deleteFederation('${f.TrustDomain}')">Delete</button>
                    </td>
                </tr>
            `;
        });
    } catch (e) {
        alert("Failed to load federations.");
    }
}

async function showFederationInfo(td) {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/federations/info?trust_domain=${encodeURIComponent(td)}`);
        const data = await res.json();

        let profileType = "Unknown";
        let spiffeID = "N/A";
        if (data.https_spiffe || data.httpsSpiffe) {
            profileType = "HTTPS SPIFFE";
            const prof = data.https_spiffe || data.httpsSpiffe;
            spiffeID = prof.endpoint_spiffe_id || prof.endpointSpiffeId || 'N/A';
        } else if (data.https_web || data.httpsWeb) {
            profileType = "HTTPS Web";
        }

        const details = `Trust Domain: ${data.trust_domain}\nBundle Endpoint: ${data.bundle_endpoint_url}\nProfile Type: ${profileType}\nSPIFFE ID: ${spiffeID}`;
        document.getElementById('info-modal-title').innerText = "Federation Relationship Info";
        document.getElementById('info-modal-text').innerText = details;
        openModal('info-modal');
    } catch (e) {
        alert("Failed to get federation info.");
    }
}

function toggleFederationProfileFields(val) {
    const container = document.getElementById('federation-spiffe-id-container');
    if (val === 'spiffe') {
        container.style.display = 'flex';
    } else {
        container.style.display = 'none';
    }
}

function switchFederationTab(tab) {
    activeFederationTab = tab;
    document.querySelectorAll('#federation-tabs .tab').forEach(el => el.classList.remove('active'));
    document.getElementById(`tab-federation-${tab}`).classList.add('active');

    document.querySelectorAll('#federation-modal .tab-content').forEach(el => el.classList.remove('active'));
    document.getElementById(`pane-federation-${tab}`).classList.add('active');

    const confirmBtn = document.getElementById('federation-confirm-btn');
    if (tab === 'internal') {
        confirmBtn.innerText = "Federate";
        const innerSelect = document.getElementById('internal-server-select');
        confirmBtn.disabled = !innerSelect;
    } else {
        confirmBtn.innerText = "Save";
        confirmBtn.disabled = false;
    }
}

async function populateInternalServers() {
    try {
        const res = await apiFetch('/api/servers');
        const servers = await res.json();

        const fedRes = await apiFetch(`/api/servers/${activeServerId}/federations`);
        const feds = await fedRes.json();

        const titleNode = document.getElementById('server-sidebar-title');
        const spanNode = titleNode ? titleNode.querySelector('span:last-child') : null;
        const currentDomain = spanNode ? spanNode.innerText.trim() : "";

        const alreadyFederated = {};
        feds.forEach(f => {
            alreadyFederated[f.TrustDomain] = true;
        });

        const eligible = servers.filter(s => {
            return s.id !== activeServerId &&
                s.domain &&
                s.domain !== 'Unknown' &&
                s.domain !== currentDomain &&
                !alreadyFederated[s.domain];
        });

        const paneInternal = document.getElementById('pane-federation-internal');

        if (eligible.length === 0) {
            paneInternal.innerHTML = `
                <div class="form-group" style="padding: 10px 0; text-align: center; color: var(--muted);">
                    No other eligible internal servers found for federation.
                </div>
            `;
            if (activeFederationTab === 'internal') {
                document.getElementById('federation-confirm-btn').disabled = true;
            }
        } else {
            paneInternal.innerHTML = `
                <div class="form-group">
                    <label for="internal-server-select">Select Server:</label>
                    <select id="internal-server-select"></select>
                </div>
            `;
            const innerSelect = document.getElementById('internal-server-select');
            eligible.forEach(s => {
                innerSelect.innerHTML += `<option value="${s.id}">${s.name} (${s.domain})</option>`;
            });
            if (activeFederationTab === 'internal') {
                document.getElementById('federation-confirm-btn').disabled = false;
            }
        }
    } catch (e) {
        console.error("Failed to load internal servers:", e);
    }
}

async function openFederationModal(td) {
    const isUpdate = (td !== null);
    document.getElementById('federation-modal-title').innerText = isUpdate ? "Update Federation Relationship" : "New Federation Relationship";
    const tdContainer = document.getElementById('federation-td-container');
    const tabsContainer = document.getElementById('federation-tabs');

    if (isUpdate) {
        tabsContainer.style.display = 'none';
        switchFederationTab('external');

        tdContainer.style.display = 'none';
        document.getElementById('federation-trust-domain').value = td;

        try {
            const res = await apiFetch(`/api/servers/${activeServerId}/federations/info?trust_domain=${encodeURIComponent(td)}`);
            const data = await res.json();

            document.getElementById('federation-endpoint-url').value = data.bundleEndpointUrl || data.bundle_endpoint_url || '';

            if (data.httpsSpiffe || data.https_spiffe) {
                document.getElementById('federation-profile-type').value = 'spiffe';
                const prof = data.httpsSpiffe || data.https_spiffe;
                document.getElementById('federation-spiffe-id').value = prof.endpointSpiffeId || prof.endpoint_spiffe_id || '';
                document.getElementById('federation-spiffe-id-container').style.display = 'flex';
            } else {
                document.getElementById('federation-profile-type').value = 'web';
                document.getElementById('federation-spiffe-id').value = '';
                document.getElementById('federation-spiffe-id-container').style.display = 'none';
            }
        } catch (e) {
            alert("Failed to load existing federation relationship details.");
        }
    } else {
        tabsContainer.style.display = 'flex';
        await populateInternalServers();
        switchFederationTab('internal');

        tdContainer.style.display = 'flex';
        document.getElementById('federation-trust-domain').value = '';
        document.getElementById('federation-endpoint-url').value = '';
        document.getElementById('federation-profile-type').value = 'spiffe';
        document.getElementById('federation-spiffe-id').value = '';
        document.getElementById('federation-spiffe-id-container').style.display = 'flex';
    }
    openModal('federation-modal');
}

async function submitFederation() {
    const confirmBtn = document.getElementById('federation-confirm-btn');
    const originalText = confirmBtn.innerText;

    if (activeFederationTab === 'internal') {
        const selectEl = document.getElementById('internal-server-select');
        if (!selectEl) return;
        const targetServerId = selectEl.value;
        if (!targetServerId) {
            alert("Please select a target server.");
            return;
        }

        try {
            confirmBtn.innerText = "Federating...";
            confirmBtn.disabled = true;

            const res = await apiFetch(`/api/servers/${activeServerId}/federations/internal`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ target_server_id: parseInt(targetServerId) })
            });

            confirmBtn.innerText = originalText;
            confirmBtn.disabled = false;

            if (res.ok) {
                closeModal('federation-modal');
                loadServerFederations();
            } else {
                const d = await res.json();
                alert(d.error || "Failed");
            }
        } catch (e) {
            confirmBtn.innerText = originalText;
            confirmBtn.disabled = false;
            alert("Failed to establish internal federation.");
        }
    } else {
        const td = document.getElementById('federation-trust-domain').value;
        const url = document.getElementById('federation-endpoint-url').value;
        const profile = document.getElementById('federation-profile-type').value;
        const spiffeId = document.getElementById('federation-spiffe-id').value;

        if (!td || !url) {
            alert("Please fill all federation fields.");
            return;
        }
        if (profile === 'spiffe' && !spiffeId) {
            alert("Please specify the Endpoint SPIFFE ID for the SPIFFE profile.");
            return;
        }
        const isUpdate = document.getElementById('federation-modal-title').innerText.indexOf("Update") !== -1;
        try {
            confirmBtn.innerText = isUpdate ? "Saving..." : "Creating...";
            confirmBtn.disabled = true;

            const res = await apiFetch(`/api/servers/${activeServerId}/federations`, {
                method: isUpdate ? 'PUT' : 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    trust_domain: td,
                    endpoint_url: url,
                    profile_type: profile,
                    endpoint_spiffe_id: spiffeId
                })
            });

            confirmBtn.innerText = originalText;
            confirmBtn.disabled = false;

            if (res.ok) {
                closeModal('federation-modal');
                loadServerFederations();
            } else {
                const d = await res.json();
                alert(d.error || "Failed");
            }
        } catch (e) {
            confirmBtn.innerText = originalText;
            confirmBtn.disabled = false;
        }
    }
}

async function refreshFederation(td) {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/federations/refresh?trust_domain=${encodeURIComponent(td)}`, {
            method: 'POST'
        });
        if (res.ok) {
            alert("Bundle refreshed successfully.");
            loadServerFederations();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}

async function deleteFederation(td) {
    if (!confirm(`Delete relationship with ${td}?`)) return;
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/federations?trust_domain=${encodeURIComponent(td)}`, {
            method: 'DELETE'
        });
        if (res.ok) {
            loadServerFederations();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}
