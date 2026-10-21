# 🔐 Microsoft Entra ID (OIDC) Setup & Testing Guide

This guide provides step-by-step instructions for creating a free **Microsoft Entra ID (Azure Active Directory)** tenant, registering and configuring the **`spire-admin-web`** application, configuring Role-Based Access Control (RBAC) for administrators, and running end-to-end authentication tests.

---

## Part 1: Create a Free Microsoft Entra ID Tenant

You can create a standalone cloud tenant in seconds without providing a credit card.

1. Navigate to the [Microsoft Entra Admin Center](https://entra.microsoft.com).
2. Sign in with any personal Microsoft account (`@outlook.com`, `@hotmail.com`, etc.).
3. If you do not have an existing tenant:
   - In the search bar at the very top, type **`Manage tenants`** and select it.
   - Click **+ Create**.
   - Select **Microsoft Entra ID** and click **Next: Configuration**.
   - Fill in:
     - **Organization name**: `SpireDev` (or your preferred name)
     - **Initial domain name**: `spiredev<unique-number>.onmicrosoft.com`
     - **Country/Region**: Select your region
   - Click **Review + create** &rarr; **Create**.
4. Once created, click the link to switch to your new directory, or use the tenant switcher icon in the top right.

> [!TIP]
> **Fast Navigation Tip**: Microsoft frequently updates navigation blades. If you cannot find a menu item, use the global search bar at the top of the portal and type **`App registrations`**, or open the direct link: [https://entra.microsoft.com/#view/Microsoft_AAD_IAM/ActiveDirectoryMenuBlade/~/RegisteredApps](https://entra.microsoft.com/#view/Microsoft_AAD_IAM/ActiveDirectoryMenuBlade/~/RegisteredApps).

---

## Part 2: Register & Configure `spire-admin-web`

### 1. Register the Application

1. In the Microsoft Entra Admin Center, search for and open **App registrations**.
2. Click **+ New registration**.
3. Configure the fields:
   - **Name**: `SPIRE Admin Web Console`
   - **Supported account types**: `Accounts in this organizational directory only (Single tenant)`
   - **Redirect URI**:
     - Platform: Select **Web**
     - URI: `https://localhost:8443/api/auth/callback`
4. Click **Register**.

---

### 2. Record Client & Tenant Identifiers

On the application's **Overview** page, copy and save:

- **Application (client) ID**: (e.g. `11111111-2222-3333-4444-555555555555`)
- **Directory (tenant) ID**: (e.g. `aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee`)

---

### 3. Generate a Client Secret

1. In the left navigation, select **Certificates & secrets** &rarr; **Client secrets** tab.
2. Click **+ New client secret**.
3. Enter a description (e.g., `spire-web-local-dev`) and choose an expiration (e.g., `180 days`).
4. Click **Add**.
5. **Immediately copy the `Value` column** *(this value is only displayed once)*.

---

### 4. Configure App Roles (Admin Privilege Mapping)

The cleanest zero-trust mechanism in Entra ID is declaring application roles rather than coupling your code to directory group GUIDs:

1. In the left menu, select **App roles** &rarr; click **+ Create app role**.
2. Fill in the role configuration:
   - **Display name**: `SPIRE Administrator`
   - **Allowed member types**: `Users/Groups`
   - **Value**: `admin` *(This exact value is checked by the Go backend to set `is_admin = true`)*
   - **Description**: `Full administrative access to SPIRE Admin Console`
   - Check **Do you want to enable this app role?**
3. Click **Apply**.

---

### 5. Assign the Admin Role to Your Test User

1. In the top search bar, navigate to **Enterprise applications** (or select **Identity** &rarr; **Applications** &rarr; **Enterprise applications**).
2. Click on **SPIRE Admin Web Console** from the application list.
3. In the left menu, select **Users and groups** &rarr; click **+ Add user/group**.
4. Click **Users (None Selected)** and select your account.
5. Click **Select a role (None Selected)** and choose **`SPIRE Administrator`**.
6. Click **Select** &rarr; **Assign**.

---

## Part 3: Configure Environment Variables

Export your Microsoft Entra ID credentials in your shell before launching the server:

```bash
# Required Microsoft Entra ID Configuration
export OIDC_TENANT_ID="<your-directory-tenant-id>"
export OIDC_CLIENT_ID="<your-application-client-id>"
export OIDC_CLIENT_SECRET="<your-client-secret-value>"
export OIDC_ADMIN_ROLE="admin"

# Optional Settings (Defaults shown)
# export OIDC_REDIRECT_URL="https://localhost:8443/api/auth/callback"
# export OIDC_ADMIN_GROUP="SPIRE-Admins"
```

| Environment Variable | CLI Flag Equivalent | Required | Description |
| :--- | :--- | :---: | :--- |
| `OIDC_TENANT_ID` | `-oidc-tenant-id` | **Yes** | Microsoft Entra Directory (Tenant) GUID |
| `OIDC_CLIENT_ID` | `-oidc-client-id` | **Yes** | Microsoft Entra Application (Client) GUID |
| `OIDC_CLIENT_SECRET` | `-oidc-client-secret` | **Yes** | Client Secret Value generated under Certificates & secrets |
| `OIDC_ADMIN_ROLE` | `-oidc-admin-role` | No | App Role value mapped to console admin (default: `admin`) |
| `OIDC_REDIRECT_URL` | `-oidc-redirect-url` | No | Callback URL (default: `https://localhost:<port>/api/auth/callback`) |
| `OIDC_ADMIN_GROUP` | `-oidc-admin-group` | No | Entra Security Group display name or GUID (default: `SPIRE-Admins`) |

---

## Part 4: Testing & Verification Steps

### 1. Build and Start the Web Console

```bash
# 1. Build the binary
go build -o spire-admin-web ./apps/spire-admin-web

# 2. Run pointing to your SPIRE agent socket and TLS certificates
./spire-admin-web \
  -socket /tmp/spire-data/spire-admin/agent.sock \
  -cert /tmp/spire-certs/cert.pem \
  -key /tmp/spire-certs/key.pem \
  -port 8443
```

Check the startup log output to confirm OIDC initialization:

```text
[INFO] OIDC authentication enabled (Client ID: <your-client-id>, Redirect URL: https://localhost:8443/api/auth/callback)
```

---

### 2. Verify in the Browser

1. Open your browser and navigate to:
   ```text
   https://localhost:8443
   ```
2. You will be redirected to `login.html`. Notice the **Sign in with Microsoft** button rendered above the standard credentials form.
3. Click **Sign in with Microsoft**:
   - The browser redirects to `https://login.microsoftonline.com/<tenant-id>/oauth2/v2.0/authorize?...`.
   - Log in with your Entra ID tenant user account.
   - Accept the consent prompt for standard profile permissions (`openid`, `profile`, `email`) if prompted.
4. After authentication, Microsoft redirects back to `https://localhost:8443/api/auth/callback`:
   - The backend validates the cryptographic signature of the token against Microsoft's public JWKS.
   - It matches your user's role against `OIDC_ADMIN_ROLE="admin"`.
   - You are redirected to `dashboard.html`.
5. Verify privileges in `dashboard.html`:
   - The **👑 User Manager** tab is visible at the bottom left.
   - Server administration, key rotation, and entry management actions are fully authorized.

---

## Part 5: Troubleshooting Common Issues

### 1. `AADSTS50011: The redirect URI specified in the request does not match`

- **Cause**: The redirect URI registered in Entra ID does not match the URL configured in the app.
- **Fix**: Verify that **App registrations** &rarr; **Authentication** &rarr; **Web** contains exactly:
  `https://localhost:8443/api/auth/callback` (ensure `https`, correct port, and no trailing slash).

### 2. User is Authenticated but Not an Administrator

- **Cause**: The user account has not been assigned the `admin` App Role.
- **Fix**:
  1. Open **Enterprise applications** &rarr; **SPIRE Admin Web Console** &rarr; **Users and groups**.
  2. Confirm your user is listed with the role **SPIRE Administrator**.
  3. Ensure your server environment variable is set to `export OIDC_ADMIN_ROLE="admin"`.

### 3. Untrusted Localhost TLS Warning

- **Cause**: When running locally with `./generate_certs.sh`, the certificate is self-signed.
- **Fix**: Import `/tmp/spire-certs/rootCA.pem` into your browser/OS trusted root certificate store, or click "Advanced" &rarr; "Proceed to localhost" in the browser for local testing.
