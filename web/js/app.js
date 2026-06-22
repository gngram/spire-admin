// Shared API, Auth and Theme Logic

// Theme Management
function getTheme() {
    return localStorage.getItem('spire_theme') || 'Purple';
}

function setTheme(theme) {
    localStorage.setItem('spire_theme', theme);
    applyTheme(theme);
}

function applyTheme(theme) {
    const body = document.body;
    // Remove other theme classes
    body.classList.remove('theme-purple', 'theme-green', 'theme-blue', 'theme-gray');
    body.classList.add('theme-' + theme.toLowerCase());
}

// Initialize theme on page load
document.addEventListener('DOMContentLoaded', () => {
    applyTheme(getTheme());
});

// Authentication
function getToken() { return sessionStorage.getItem('spire_token'); }
function setToken(token) { sessionStorage.setItem('spire_token', token); }
function isAdmin() { return sessionStorage.getItem('spire_admin') === 'true'; }

function checkAuth() {
    if (!getToken()) {
        window.location.href = 'login.html';
    } else {
        // Setup nav UI for admin
        const adminLink = document.getElementById('nav-admin');
        if (adminLink) {
            if (isAdmin()) {
                adminLink.style.display = 'flex';
            } else {
                adminLink.style.display = 'none';
            }
        }
        setupInactivityTimer();
    }
}

async function logout() {
    const token = getToken();
    if (token) {
        try {
            await fetch('/api/logout', {
                method: 'POST',
                headers: {
                    'Authorization': 'Bearer ' + token
                }
            });
        } catch (e) {
            console.error("Logout request failed:", e);
        }
    }
    sessionStorage.removeItem('spire_token');
    sessionStorage.removeItem('spire_admin');
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
        // Logout after 30 minutes of inactivity
        timeout = setTimeout(logout, 30 * 60 * 1000);
    };

    window.onload = resetTimer;
    document.onmousemove = resetTimer;
    document.onkeydown = resetTimer;
}
