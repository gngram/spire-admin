// Trust Bundles Actions
async function loadServerBundles() {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/bundles`);
        const data = await res.json();
        const tbody = document.querySelector('#bundles-table tbody');
        tbody.innerHTML = '';
        if (!data || data.length === 0) {
            tbody.innerHTML = `<tr><td colspan="2" style="text-align: center; color: var(--muted);">No federated bundles.</td></tr>`;
            return;
        }
        data.forEach(b => {
            tbody.innerHTML += `
                <tr>
                    <td style="font-weight: bold;">${b.trust_domain}</td>
                    <td style="text-align: right;">
                        <button class="btn btn-secondary" onclick="showBundleInfo('${b.trust_domain}')">Info</button>
                        <button class="btn btn-secondary" onclick="openSetBundleModal('${b.trust_domain}')">Update</button>
                        <button class="btn btn-secondary btn-danger" onclick="deleteBundle('${b.trust_domain}')">Delete</button>
                    </td>
                </tr>
            `;
        });
    } catch (e) {
        alert("Failed to load bundles.");
    }
}

async function showBundleInfo(td) {
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/bundles/info?trust_domain=${encodeURIComponent(td)}`);
        const data = await res.json();

        let text = `Trust Domain  : ${data.trust_domain}\nSequence      : ${data.sequence_number || 0}\nRefresh Hint  : ${data.refresh_hint || 0} seconds\n`;
        if (data.x509_authorities) {
            text += `\nX.509 Authorities (${data.x509_authorities.length}):\n`;
            data.x509_authorities.forEach((auth, idx) => {
                text += `  [${idx}] Certificate Payload (DER/ASN.1) encoded length: ${auth.asn1 ? atob(auth.asn1).length : 0} bytes\n`;
            });
        }
        if (data.jwt_authorities) {
            text += `\nJWT Authorities (${data.jwt_authorities.length}):\n`;
            data.jwt_authorities.forEach((auth, idx) => {
                text += `  [${idx}] Key ID: ${auth.key_id || auth.keyId || ''}\n`;
            });
        }

        document.getElementById('info-modal-title').innerText = "Bundle Details: " + td;
        document.getElementById('info-modal-text').innerText = text;
        openModal('info-modal');
    } catch (e) {
        alert("Failed to get bundle info.");
    }
}

function openSetBundleModal(td) {
    const isUpdate = (td !== null);
    document.getElementById('bundle-modal-title').innerText = isUpdate ? "Update Federated Bundle" : "Set Federated Bundle";
    const tdContainer = document.getElementById('bundle-td-container');
    if (isUpdate) {
        tdContainer.style.display = 'none';
        document.getElementById('bundle-trust-domain').value = td;
    } else {
        tdContainer.style.display = 'flex';
        document.getElementById('bundle-trust-domain').value = '';
    }
    document.getElementById('bundle-pem-content').value = '';
    openModal('bundle-modal');
}

async function submitSetBundle() {
    const td = document.getElementById('bundle-trust-domain').value;
    const pem = document.getElementById('bundle-pem-content').value;
    if (!td || !pem) {
        alert("Please fill all bundle fields.");
        return;
    }
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/bundles`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ trust_domain: td, pem_content: pem })
        });
        if (res.ok) {
            closeModal('bundle-modal');
            loadServerBundles();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}

async function deleteBundle(td) {
    if (!confirm(`Delete federated bundle for ${td}?`)) return;
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/bundles?trust_domain=${encodeURIComponent(td)}`, {
            method: 'DELETE'
        });
        if (res.ok) {
            loadServerBundles();
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}
