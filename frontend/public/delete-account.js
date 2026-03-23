async function checkAuth() {
  try {
    const res = await fetch('/api/v1/auth/me', { credentials: 'include' });
    document.getElementById('loading').style.display = 'none';
    if (res.ok) {
      const data = await res.json();
      document.getElementById('user-email').textContent = data.email;
      document.getElementById('logged-in').style.display = 'block';
    } else {
      document.getElementById('not-logged-in').style.display = 'block';
    }
  } catch {
    document.getElementById('loading').style.display = 'none';
    document.getElementById('not-logged-in').style.display = 'block';
  }
}

async function confirmDelete() {
  if (!confirm('Are you absolutely sure? This cannot be undone.')) return;

  const btn = document.getElementById('delete-btn');
  const status = document.getElementById('status');
  btn.disabled = true;
  btn.textContent = 'Deleting...';

  try {
    const res = await fetch('/api/v1/account', {
      method: 'DELETE',
      credentials: 'include',
    });

    if (res.ok) {
      document.getElementById('logged-in').style.display = 'none';
      status.textContent = 'Your account has been deleted. You can close this page.';
      status.className = 'status-success';
    } else {
      const data = await res.text();
      status.textContent = 'Failed to delete account: ' + data;
      status.className = 'status-error';
      btn.disabled = false;
      btn.textContent = 'Delete my account';
    }
  } catch (err) {
    status.textContent = 'Network error. Please try again.';
    status.className = 'status-error';
    btn.disabled = false;
    btn.textContent = 'Delete my account';
  }
}

// Wire up event listeners (replaces inline onclick attributes)
document.getElementById('sign-in-btn').addEventListener('click', function () {
  window.location.href = '/';
});

document.getElementById('delete-btn').addEventListener('click', confirmDelete);

document.getElementById('cancel-btn').addEventListener('click', function () {
  window.location.href = '/';
});

// Check auth on load
checkAuth();
