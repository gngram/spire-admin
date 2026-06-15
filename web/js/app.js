
// Shared API and Auth Logic
function getToken() { return localStorage.getItem('spire_token'); }
function setToken(token) { localStorage.setItem('spire_token', token); }
function isAdmin() { return localStorage.getItem('spire_admin') === 'true'; }

function checkAuth() {
    if (!getToken()) {
        window.location.href = 'login.html';
    } else {
        // Initialize Nav UI based on role
        const adminLink = document.getElementById('nav-admin');
        if (adminLink && isAdmin()) {
            adminLink.style.display = 'inline-block';
        }
        setupInactivityTimer();
    }
}

function logout() {
    localStorage.removeItem('spire_token');
    localStorage.removeItem('spire_admin');
    window.location.href = 'login.html';
}

async function apiFetch(url, options = {}) {
    options.headers = options.headers || {};
    const token = getToken();
    if (token) {
        options.headers['Authorization'] = 'Bearer ' + token;
    }
    const res = await fetch(url, options);
    if (res.status === 401) {
        logout(); // Auto-logout on token expiration
    }
    return res;
}

function setupInactivityTimer() {
    let timeout;
    const resetTimer = () => {
        clearTimeout(timeout);
        // Logout after 30 minutes of no mouse/keyboard interaction
        timeout = setTimeout(logout, 30 * 60 * 1000); 
    };

    window.onload = resetTimer;
    document.onmousemove = resetTimer;
    document.onkeydown = resetTimer;
}
