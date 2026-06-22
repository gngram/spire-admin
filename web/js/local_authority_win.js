// Local Authorities Actions
async function loadLocalAuthorities() {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/local-authority`);
        const data = await res.json();

        const activeBox = document.getElementById('local-auth-active');
        const preparedBox = document.getElementById('local-auth-prepared');
        const oldBox = document.getElementById('local-auth-old');

        const renderState = (state, label) => {
            if (!state) return `No ${label.toLowerCase()} X.509 authority found`;
            const dateStr = new Date(state.expires_at * 1000).toUTCString();
            let actBtn = '';
            if (label === 'Prepared') {
                actBtn = `<button class="btn btn-success" style="margin-top: 10px;" onclick="activateLocalAuthority('${state.authority_id}')">Activate</button>`;
            } else if (label === 'Old') {
                actBtn = `<button class="btn btn-danger" style="margin-top: 10px;" onclick="deleteLocalAuthority('${state.authority_id}')">Delete (Taint + Revoke)</button>`;
            }
            return `Authority ID         : ${state.authority_id}\nExpires at           : ${dateStr}\nUpstream authority ID: No upstream authority\n${actBtn}`;
        };

        activeBox.innerHTML = renderState(data.active, "Active");
        preparedBox.innerHTML = renderState(data.prepared, "Prepared");
        oldBox.innerHTML = renderState(data.old, "Old");

    } catch (e) {
        alert("Failed to load local authorities.");
    }
}

async function rotateLocalAuthority() {
    if (!confirm("Prepare and activate a new X.509 authority?")) return;
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/local-authority/rotate`, { method: 'POST' });
        if (res.ok) {
            loadLocalAuthorities();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}

async function activateLocalAuthority(authID) {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/local-authority/activate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ authority_id: authID })
        });
        if (res.ok) {
            loadLocalAuthorities();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}

async function deleteLocalAuthority(authID) {
    if (!confirm(`Are you sure you want to taint and revoke old authority ${authID}?`)) return;
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/local-authority/delete`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ authority_id: authID })
        });
        if (res.ok) {
            loadLocalAuthorities();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}
