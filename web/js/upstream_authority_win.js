// Upstream Authority Actions
async function upstreamAction(type) {
    const skid = document.getElementById('upstream-skid').value;
    if (!skid) {
        alert("Please provide Upstream Subject Key ID.");
        return;
    }
    try {
        const res = await apiFetch(`/api/servers/${activeServerId}/upstream-authority/${type}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ subject_key_id: skid })
        });
        if (res.ok) {
            alert(`Upstream authority successfully ${type}ed.`);
            document.getElementById('upstream-skid').value = '';
        } else {
            const d = await res.json();
            alert(d.error || "Failed");
        }
    } catch (e) { }
}
