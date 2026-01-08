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

    <h1>Privacy Policy</h1>
    <p class="last-updated">Last updated: January 2026</p>

    <section>
      <h2>Introduction</h2>
      <p>
        tiny tracker ("we", "our", or "us") is committed to protecting your privacy.
        This Privacy Policy explains how we collect, use, and safeguard your information
        when you use our goal tracking application.
      </p>
    </section>

    <section>
      <h2>Information We Collect</h2>
      <h3>Account Information</h3>
      <p>
        When you sign in with Google, we receive and store:
      </p>
      <ul>
        <li>Your Google account email address</li>
        <li>Your display name</li>
        <li>Your profile picture URL</li>
      </ul>

      <h3>Goal and Completion Data</h3>
      <p>
        We store the goals you create and your daily completion records to provide
        the core functionality of the application.
      </p>

      <h3>Guest Mode (Local Storage)</h3>
      <p>
        If you use the application without signing in, your data is stored locally
        in your browser and is not transmitted to our servers.
      </p>
    </section>

    <section>
      <h2>How We Use Your Information</h2>
      <p>We use your information to:</p>
      <ul>
        <li>Provide and maintain the goal tracking service</li>
        <li>Sync your data across devices when you sign in</li>
        <li>Improve and optimize the application</li>
      </ul>
    </section>

    <section>
      <h2>Data Storage and Security</h2>
      <p>
        Your data is stored on secure servers using PostgreSQL databases.
        We implement appropriate technical and organizational measures to
        protect your personal information against unauthorized access,
        alteration, disclosure, or destruction.
      </p>
    </section>

    <section>
      <h2>Data Sharing</h2>
      <p>
        We do not sell, trade, or share your personal information with third parties,
        except as required by law or to protect our rights.
      </p>
    </section>

    <section>
      <h2>Your Rights</h2>
      <p>You have the right to:</p>
      <ul>
        <li>Access the personal data we hold about you</li>
        <li>Request deletion of your data</li>
        <li>Export your data</li>
        <li>Withdraw consent at any time by deleting your account</li>
      </ul>
    </section>

    <section>
      <h2>Cookies and Local Storage</h2>
      <p>
        We use essential cookies for authentication purposes.
        Local storage is used to maintain guest mode data and application preferences.
      </p>
    </section>

    <section>
      <h2>Changes to This Policy</h2>
      <p>
        We may update this Privacy Policy from time to time.
        We will notify you of any changes by posting the new Privacy Policy on this page
        and updating the "Last updated" date.
      </p>
    </section>

    <section>
      <h2>Contact Us</h2>
      <p>
        If you have any questions about this Privacy Policy, please contact us through
        the application's support channels.
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

  h3 {
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 16px 0 8px;
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
