// Current UI State
let activeServerId = null;
let activeServerName = "";
let logInterval = null;

// Switch tabs in sidebar
function switchMainTab(tab) {
    // Stop logging interval if leaving logs
    if (logInterval) {
        clearInterval(logInterval);
        logInterval = null;
    }

    // Set sidebar active styles
    document.querySelectorAll('#main-sidebar .nav-item').forEach(el => el.classList.remove('active'));
    if (typeof event !== 'undefined' && event && event.currentTarget) {
        event.currentTarget.classList.add('active');
    }

    // Hide all tab panes
    document.querySelectorAll('.tab-pane').forEach(el => el.style.display = 'none');
    document.getElementById('server-workspace').style.display = 'none';

    // Show selected pane
    const titleEl = document.getElementById('page-title');
    const subtitleEl = document.getElementById('page-subtitle');
    const actionsEl = document.getElementById('header-actions');
    actionsEl.innerHTML = '';

    if (tab === 'servers') {
        titleEl.innerText = "Servers";
        subtitleEl.innerText = "Manage the list of Spire servers you want to administrate.";
        document.getElementById('view-servers').style.display = 'block';
        loadServers();
    } else if (tab === 'settings') {
        titleEl.innerText = "Settings";
        subtitleEl.innerText = "Application settings and preferences.";
        document.getElementById('view-settings').style.display = 'block';
        document.getElementById('theme-selector').value = getTheme();
    } else if (tab === 'logs') {
        titleEl.innerText = "Logs";
        subtitleEl.innerText = "View application events and system logs.";
        document.getElementById('view-logs').style.display = 'block';
        loadLogs();
        logInterval = setInterval(loadLogs, 3000);
    } else if (tab === 'about') {
        titleEl.innerText = "About";
        subtitleEl.innerText = "";
        document.getElementById('view-about').style.display = 'block';
    } else if (tab === 'admin') {
        titleEl.innerText = "User Manager";
        subtitleEl.innerText = "Provision accounts for administrators and operators.";
        document.getElementById('view-admin').style.display = 'block';
    }
}

// Provision User account
async function adminCreateUser() {
    const u = document.getElementById('admin-new-user').value;
    const p = document.getElementById('admin-new-pass').value;
    const isAdm = document.getElementById('admin-is-admin').checked;
    if (!u || !p) {
        alert("Please fill username and password fields.");
        return;
    }
    try {
        const res = await apiFetch('/api/users', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username: u, password: p, is_admin: isAdm })
        });
        const data = await res.json();
        document.getElementById('admin-msg').innerText = data.status || data.error || "Done.";
        if (res.ok) {
            document.getElementById('admin-new-user').value = '';
            document.getElementById('admin-new-pass').value = '';
            document.getElementById('admin-is-admin').checked = false;
        }
    } catch (e) {
        document.getElementById('admin-msg').innerText = "Request failed.";
    }
}

// Server Details Navigation
function enterServerView(id, name) {
    activeServerId = id;
    activeServerName = name;

    // Switch sidebar navigation panel
    document.getElementById('main-sidebar').style.display = 'none';
    document.getElementById('server-sidebar').style.display = 'flex';
    document.getElementById('server-sidebar-title').querySelector('span:last-child').innerText = name;

    // Hide main panels
    document.querySelectorAll('.tab-pane').forEach(el => el.style.display = 'none');
    document.getElementById('server-workspace').style.display = 'flex';

    // Select default tab
    switchServerTab('agents');
}

function exitServerView() {
    activeServerId = null;
    activeServerName = "";

    // Switch sidebar back
    document.getElementById('server-sidebar').style.display = 'none';
    document.getElementById('main-sidebar').style.display = 'flex';

    // Reset navigation header style
    document.querySelectorAll('#main-sidebar .nav-item').forEach(el => el.classList.remove('active'));
    document.querySelector('#main-sidebar .nav-item:first-child').classList.add('active');

    // Reset titles and display
    switchMainTab('servers');
}

function switchServerTab(tab) {
    // Highlight nav item
    document.querySelectorAll('#server-sidebar .nav-item').forEach(el => el.classList.remove('active'));
    if (typeof event !== 'undefined' && event && event.currentTarget) {
        event.currentTarget.classList.add('active');
    }

    // Hide all sub-panes
    document.querySelectorAll('.server-tab-pane').forEach(el => el.style.display = 'none');

    const titleEl = document.getElementById('page-title');
    const subtitleEl = document.getElementById('page-subtitle');
    const actionsEl = document.getElementById('header-actions');
    actionsEl.innerHTML = '';

    if (tab === 'agents') {
        titleEl.innerText = "Agents";
        subtitleEl.innerText = "Manage SPIRE agents connected to " + activeServerName + ".";
        actionsEl.innerHTML = `
            <button class="btn btn-secondary" onclick="loadServerAgents()">Refresh</button>
            <button class="btn btn-danger" onclick="purgeExpiredAgents()">Purge Expired</button>
        `;
        document.getElementById('server-pane-agents').style.display = 'block';
        loadServerAgents();
    } else if (tab === 'bundles') {
        titleEl.innerText = "Federated Bundles";
        subtitleEl.innerText = "Manage federated trust bundles in " + activeServerName + ".";
        actionsEl.innerHTML = `
            <button class="btn btn-secondary" onclick="loadServerBundles()">Refresh</button>
            <button class="btn btn-primary" onclick="openSetBundleModal(null)">New</button>
        `;
        document.getElementById('server-pane-bundles').style.display = 'block';
        loadServerBundles();
    } else if (tab === 'federations') {
        titleEl.innerText = "Federation Relationships";
        subtitleEl.innerText = "Manage dynamic federation relationships with foreign trust domains.";
        actionsEl.innerHTML = `
            <button class="btn btn-secondary" onclick="loadServerFederations()">Refresh</button>
            <button class="btn btn-primary" onclick="openFederationModal(null)">New</button>
        `;
        document.getElementById('server-pane-federations').style.display = 'block';
        loadServerFederations();
    } else if (tab === 'entries') {
        titleEl.innerText = "Entries";
        subtitleEl.innerText = "Manage registered workload/agent entries.";
        actionsEl.innerHTML = `
            <button class="btn btn-secondary" onclick="loadServerEntries()">Refresh</button>
            <button class="btn btn-primary" onclick="openCreateEntryModal()">New</button>
        `;
        document.getElementById('server-pane-entries').style.display = 'block';
        loadServerEntries();
    } else if (tab === 'local-auth') {
        titleEl.innerText = "Local X.509 Authorities";
        subtitleEl.innerText = "Manage local signing authorities and rotation.";
        actionsEl.innerHTML = `
            <button class="btn btn-secondary" onclick="loadLocalAuthorities()">Refresh</button>
            <button class="btn btn-primary" onclick="rotateLocalAuthority()">New</button>
        `;
        document.getElementById('server-pane-local-auth').style.display = 'block';
        loadLocalAuthorities();
    } else if (tab === 'upstream-auth') {
        titleEl.innerText = "Upstream Authority";
        subtitleEl.innerText = "Manage upstream X.509 authority trust.";
        document.getElementById('server-pane-upstream-auth').style.display = 'block';
    }
}

// Modals management
function openModal(id) {
    document.getElementById(id).style.display = 'flex';
}
function closeModal(id) {
    document.getElementById(id).style.display = 'none';
}

// Initial load
document.addEventListener('DOMContentLoaded', () => {
    checkAuth();
    loadServers();
});
