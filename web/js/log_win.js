// Load System Logs
async function loadLogs() {
    try {
        const res = await apiFetch('/api/logs');
        const data = await res.json();
        const logBox = document.getElementById('log-console-box');
        logBox.innerText = data.logs || "No logs available.";
        logBox.scrollTop = logBox.scrollHeight;
    } catch (e) { }
}
