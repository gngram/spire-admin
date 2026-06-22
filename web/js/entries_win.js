// Registered Entries Actions
let workloadEntries = [];
let agentEntries = [];
let downstreamEntries = [];
let activeEntrySubtab = "workloads";

async function loadServerEntries() {
    try {
        const [wlRes, agRes, dsRes] = await Promise.all([
            apiFetch(`/api/servers/${activeServerId}/entries/workloads`),
            apiFetch(`/api/servers/${activeServerId}/entries/agents`),
            apiFetch(`/api/servers/${activeServerId}/entries/downstreams`)
        ]);
        
        workloadEntries = await wlRes.json();
        agentEntries = await agRes.json();
        downstreamEntries = await dsRes.json();
        
        renderEntries();
    } catch (e) {
        alert("Failed to load entries.");
    }
}

function switchEntrySubTab(subtab) {
    activeEntrySubtab = subtab;
    document.querySelectorAll('#server-pane-entries .tab').forEach(el => el.classList.remove('active'));
    if (typeof event !== 'undefined' && event && event.currentTarget) {
        event.currentTarget.classList.add('active');
    }

    document.querySelectorAll('#server-pane-entries .tab-content').forEach(el => el.style.display = 'none');
    document.getElementById(`entry-subtab-${subtab}`).style.display = 'block';
    renderEntries();
}

function renderEntries() {
    const workloadsBody = document.querySelector('#entries-table-workloads tbody');
    const agentsBody = document.querySelector('#entries-table-agents tbody');
    const downstreamsBody = document.querySelector('#entries-table-downstreams tbody');

    workloadsBody.innerHTML = '';
    agentsBody.innerHTML = '';
    downstreamsBody.innerHTML = '';

    const buildRow = (e) => `
        <tr>
            <td style="font-family: monospace; text-overflow: ellipsis; overflow: hidden; white-space: nowrap; max-width: 400px;">
                ${e.SPIFFEID}
            </td>
            <td style="text-align: right;">
                <button class="btn btn-secondary" onclick="showEntryInfo('${e.ID}')">Info</button>
                <button class="btn btn-secondary" onclick="openUpdateEntryModal('${e.ID}')">Update</button>
                <button class="btn btn-secondary btn-danger" onclick="deleteEntry('${e.ID}')">Delete</button>
            </td>
        </tr>
    `;

    if (!workloadEntries || workloadEntries.length === 0) {
        workloadsBody.innerHTML = `<tr><td colspan="2" style="text-align: center; color: var(--muted);">No workloads.</td></tr>`;
    } else {
        workloadEntries.forEach(e => {
            workloadsBody.innerHTML += buildRow(e);
        });
    }

    if (!agentEntries || agentEntries.length === 0) {
        agentsBody.innerHTML = `<tr><td colspan="2" style="text-align: center; color: var(--muted);">No agents.</td></tr>`;
    } else {
        agentEntries.forEach(e => {
            agentsBody.innerHTML += buildRow(e);
        });
    }

    if (!downstreamEntries || downstreamEntries.length === 0) {
        downstreamsBody.innerHTML = `<tr><td colspan="2" style="text-align: center; color: var(--muted);">No downstream servers.</td></tr>`;
    } else {
        downstreamEntries.forEach(e => {
            downstreamsBody.innerHTML += buildRow(e);
        });
    }
}

async function showEntryInfo(id) {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/entries/${id}/info`);
        const e = await res.json();

        const sId = e.spiffe_id || e.spiffeId;
        const pId = e.parent_id || e.parentId;

        const spiffe = sId ? `spiffe://${sId.trust_domain || sId.trustDomain}${sId.path}` : 'N/A';
        const parent = pId ? `spiffe://${pId.trust_domain || pId.trustDomain}${pId.path}` : 'N/A';
        const selectors = (e.selectors || []).map(s => `${s.type}:${s.value}`).join(', ');

        const dns = e.dns_names || e.dnsNames || [];
        const ttl = e.x509_svid_ttl || e.x509SvidTtl || 0;

        const text = `ID : ${e.id}\nSPIFFE ID : ${spiffe}\nParent ID : ${parent}\nSelectors : ${selectors}\nDNS Names : ${dns.join(', ')}\nTTL : ${ttl}\nDownstream : ${e.downstream || false}\nAdmin : ${e.admin || false}\nHint : ${e.hint || 'None'}`;

        document.getElementById('info-modal-title').innerText = "Entry Details";
        document.getElementById('info-modal-text').innerText = text;
        openModal('info-modal');
    } catch (err) {
        alert("Failed to get entry info.");
    }
}

function openCreateEntryModal() {
    document.getElementById('entry-spiffe-id').value = '';
    document.getElementById('entry-parent-id').value = '';
    document.getElementById('entry-selectors').value = '';
    openModal('create-entry-modal');
}

async function submitCreateEntry() {
    const spiffe = document.getElementById('entry-spiffe-id').value;
    const parent = document.getElementById('entry-parent-id').value;
    const selectorsVal = document.getElementById('entry-selectors').value;

    if (!spiffe || !parent || !selectorsVal) {
        alert("Please fill all entry creation fields.");
        return;
    }

    const selectors = selectorsVal.split(',').map(s => s.trim());

    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/entries`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ spiffe_id: spiffe, parent_id: parent, selectors: selectors })
        });
        if (res.ok) {
            closeModal('create-entry-modal');
            loadServerEntries();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}

async function openUpdateEntryModal(id) {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/entries/${id}/info`);
        const e = await res.json();

        const dns = e.dns_names || e.dnsNames || [];
        const ttl = e.x509_svid_ttl || e.x509SvidTtl || '';
        const federates = e.federates_with || e.federatesWith || [];

        document.getElementById('update-entry-id').value = e.id;
        document.getElementById('update-entry-dns').value = dns.join(', ');
        document.getElementById('update-entry-hint').value = e.hint || '';
        document.getElementById('update-entry-ttl').value = ttl;
        document.getElementById('update-entry-federates').value = federates.join(', ');
        document.getElementById('update-entry-downstream').checked = e.downstream || false;
        document.getElementById('update-entry-admin').checked = e.admin || false;

        openModal('update-entry-modal');
    } catch (err) {
        alert("Failed to fetch entry details.");
    }
}

async function submitUpdateEntry() {
    const id = document.getElementById('update-entry-id').value;
    const dns = document.getElementById('update-entry-dns').value.split(',').map(s => s.trim()).filter(Boolean);
    const hint = document.getElementById('update-entry-hint').value;
    const ttl = parseInt(document.getElementById('update-entry-ttl').value) || 0;
    const federates = document.getElementById('update-entry-federates').value.split(',').map(s => s.trim()).filter(Boolean);
    const downstream = document.getElementById('update-entry-downstream').checked;
    const admin = document.getElementById('update-entry-admin').checked;

    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/entries/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                dns_names: dns,
                hint: hint,
                ttl: ttl,
                federates_with: federates,
                downstream: downstream,
                admin: admin
            })
        });
        if (res.ok) {
            closeModal('update-entry-modal');
            loadServerEntries();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}

async function deleteEntry(id) {
    if (!confirm(`Are you sure you want to delete entry ${id}?`)) return;
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/entries/${id}`, {
            method: 'DELETE'
        });
        if (res.ok) {
            loadServerEntries();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}
