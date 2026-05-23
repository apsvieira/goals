<script lang="ts">
  // Single source of truth: frontend/public/privacy.html. Imported as a raw string
  // at build time so any update to that file flows into the in-app view without
  // a second copy of the content here. The standalone privacy.html (linked from
  // the Play Store listing) and this in-app view always stay in sync.
  import privacyHtmlSource from '../../../public/privacy.html?raw';

  export let onBack: () => void;

  // Strip everything outside <body>...</body> and drop the standalone <style>
  // block — in-app styling is driven by the surrounding card below.
  const bodyMatch = privacyHtmlSource.match(/<body[^>]*>([\s\S]*?)<\/body>/i);
  const bodyHtml = (bodyMatch?.[1] ?? privacyHtmlSource).replace(
    /<style[\s\S]*?<\/style>/gi,
    '',
  );
</script>

<div class="legal-container">
  <div class="legal-card">
    <button class="back-btn" on:click={onBack}>
      <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M19 12H5M12 19l-7-7 7-7"/>
      </svg>
      Back
    </button>

    <div class="policy-content">
      {@html bodyHtml}
    </div>
  </div>
</div>

<style>
  .legal-container {
    min-height: 100vh;
    padding: 24px;
    background: var(--bg-primary);
  }

  .legal-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 48px;
    max-width: 800px;
    margin: 0 auto;
  }

  .back-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    font-size: 14px;
    font-weight: 500;
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
    margin-bottom: 24px;
    transition: background-color 0.2s, color 0.2s;
  }

  .back-btn:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .policy-content :global(h1) {
    margin: 0 0 8px;
    font-size: 28px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .policy-content :global(.updated) {
    margin: 0 0 32px;
    color: var(--text-muted);
    font-size: 14px;
  }

  .policy-content :global(h2) {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 24px 0 12px;
  }

  .policy-content :global(h3) {
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 16px 0 8px;
  }

  .policy-content :global(p) {
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 12px;
  }

  .policy-content :global(ul) {
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 12px;
    padding-left: 24px;
  }

  .policy-content :global(li) {
    margin-bottom: 4px;
  }

  .policy-content :global(a) {
    color: var(--accent, #4CAF50);
  }

  @media (max-width: 768px) {
    .legal-card {
      padding: 24px;
    }

    .policy-content :global(h1) {
      font-size: 24px;
    }

    .policy-content :global(h2) {
      font-size: 18px;
    }
  }
</style>
