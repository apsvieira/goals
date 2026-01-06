<script lang="ts">
  export let onBack: () => void;
</script>

<div class="legal-container">
  <div class="legal-card">
    <button class="back-btn" on:click={onBack}>
      <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M19 12H5M12 19l-7-7 7-7"/>
      </svg>
      Back
    </button>

    <h1>Terms of Service</h1>
    <p class="last-updated">Last updated: January 2026</p>

    <section>
      <h2>Acceptance of Terms</h2>
      <p>
        By accessing or using Goal Tracker, you agree to be bound by these Terms of Service.
        If you do not agree to these terms, please do not use the application.
      </p>
    </section>

    <section>
      <h2>Description of Service</h2>
      <p>
        Goal Tracker is a personal goal tracking application that allows you to:
      </p>
      <ul>
        <li>Create and manage personal goals</li>
        <li>Track daily completion of your goals</li>
        <li>View progress statistics</li>
        <li>Sync data across devices (when signed in)</li>
      </ul>
    </section>

    <section>
      <h2>User Accounts</h2>
      <p>
        You may use Goal Tracker in guest mode (data stored locally) or by signing in
        with your Google account. When signing in, you agree to:
      </p>
      <ul>
        <li>Provide accurate account information</li>
        <li>Maintain the security of your account credentials</li>
        <li>Accept responsibility for all activity under your account</li>
      </ul>
    </section>

    <section>
      <h2>Acceptable Use</h2>
      <p>You agree not to:</p>
      <ul>
        <li>Use the service for any unlawful purpose</li>
        <li>Attempt to gain unauthorized access to the service or its systems</li>
        <li>Interfere with or disrupt the service or servers</li>
        <li>Upload malicious code or content</li>
        <li>Use automated systems to access the service without permission</li>
      </ul>
    </section>

    <section>
      <h2>User Content</h2>
      <p>
        You retain ownership of the goals and data you create. By using the service,
        you grant us a limited license to store and process your data solely for
        the purpose of providing the service to you.
      </p>
    </section>

    <section>
      <h2>Service Availability</h2>
      <p>
        We strive to maintain service availability but do not guarantee uninterrupted access.
        The service may be temporarily unavailable due to maintenance, updates, or
        circumstances beyond our control.
      </p>
    </section>

    <section>
      <h2>Data and Backups</h2>
      <p>
        While we take reasonable measures to protect your data, we recommend you
        maintain your own backups. We are not responsible for data loss due to
        technical failures, user error, or other circumstances.
      </p>
      <p>
        Guest mode data is stored only in your browser's local storage and is not
        backed up to our servers. Clearing your browser data will delete this information.
      </p>
    </section>

    <section>
      <h2>Termination</h2>
      <p>
        You may stop using the service at any time. We reserve the right to suspend
        or terminate accounts that violate these terms or for any other reason at
        our discretion.
      </p>
    </section>

    <section>
      <h2>Disclaimer of Warranties</h2>
      <p>
        The service is provided "as is" without warranties of any kind, either express
        or implied. We do not warrant that the service will meet your requirements
        or be error-free.
      </p>
    </section>

    <section>
      <h2>Limitation of Liability</h2>
      <p>
        To the fullest extent permitted by law, we shall not be liable for any indirect,
        incidental, special, consequential, or punitive damages arising from your use
        of the service.
      </p>
    </section>

    <section>
      <h2>Changes to Terms</h2>
      <p>
        We may modify these terms at any time. Continued use of the service after
        changes constitutes acceptance of the modified terms.
      </p>
    </section>

    <section>
      <h2>Contact</h2>
      <p>
        For questions about these Terms of Service, please contact us through the
        application's support channels.
      </p>
    </section>
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

  h1 {
    margin: 0 0 8px;
    font-size: 28px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .last-updated {
    margin: 0 0 32px;
    color: var(--text-muted);
    font-size: 14px;
  }

  section {
    margin-bottom: 24px;
  }

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 12px;
  }

  p {
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 12px;
  }

  ul {
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 12px;
    padding-left: 24px;
  }

  li {
    margin-bottom: 4px;
  }

  @media (max-width: 768px) {
    .legal-card {
      padding: 24px;
    }

    h1 {
      font-size: 24px;
    }

    h2 {
      font-size: 18px;
    }
  }
</style>
