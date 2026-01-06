<script lang="ts">
  const currentYear = new Date().getFullYear();

  function handleLinkClick(e: MouseEvent, path: string) {
    e.preventDefault();
    window.history.pushState({}, '', path);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }
</script>

<footer>
  <div class="footer-content">
    <span class="copyright">© {currentYear} Goal Tracker</span>
    <nav class="footer-links">
      <a href="/privacy" on:click={(e) => handleLinkClick(e, '/privacy')}>Privacy</a>
      <a href="/terms" on:click={(e) => handleLinkClick(e, '/terms')}>Terms</a>
      <a href="/about" on:click={(e) => handleLinkClick(e, '/about')}>About</a>
    </nav>
  </div>
</footer>

<style>
  footer {
    background: var(--bg-secondary);
    border-top: 1px solid var(--border);
    padding: 24px;
  }

  .footer-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    max-width: 1400px;
    margin: 0 auto;
  }

  .copyright {
    color: var(--text-muted);
    font-size: 14px;
  }

  .footer-links {
    display: flex;
    gap: 24px;
  }

  .footer-links a {
    color: var(--text-secondary);
    font-size: 14px;
    text-decoration: none;
  }

  .footer-links a:hover {
    color: var(--text-primary);
  }

  @media (min-width: 768px) {
    .footer-content {
      flex-direction: row;
      justify-content: space-between;
    }
  }
</style>
