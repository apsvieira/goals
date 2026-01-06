<script lang="ts">
  export let onBack: () => void;
</script>

<div class="about-container">
  <div class="about-card">
    <button class="back-btn" on:click={onBack}>
      <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M19 12H5M12 19l-7-7 7-7"/>
      </svg>
      Back
    </button>

    <h1>About Goal Tracker</h1>

    <section>
      <p class="intro">
        Goal Tracker is a simple, focused application designed to help you build
        and maintain positive habits through daily goal tracking.
      </p>
    </section>

    <section>
      <h2>Features</h2>
      <ul>
        <li>
          <strong>Simple Goal Creation</strong> - Create goals with optional weekly
          or monthly targets
        </li>
        <li>
          <strong>Daily Tracking</strong> - Mark your progress each day with a
          simple click
        </li>
        <li>
          <strong>Visual Progress</strong> - See your completion patterns at a glance
          with the calendar grid
        </li>
        <li>
          <strong>Target Tracking</strong> - Set targets like "5 times per week"
          and track your progress
        </li>
        <li>
          <strong>Guest Mode</strong> - Use the app without an account; your data
          stays in your browser
        </li>
        <li>
          <strong>Cloud Sync</strong> - Sign in with Google to sync your goals
          across devices
        </li>
      </ul>
    </section>

    <section>
      <h2>How It Works</h2>
      <ol>
        <li>Create a goal for something you want to track (exercise, reading, etc.)</li>
        <li>Optionally set a target frequency (e.g., 3x per week)</li>
        <li>Each day, click on the calendar to mark when you've completed your goal</li>
        <li>Review your progress over time and stay motivated</li>
      </ol>
    </section>

    <section>
      <h2>Privacy First</h2>
      <p>
        Your goals are personal. In guest mode, all data stays on your device.
        When signed in, your data is stored securely and is never shared with
        third parties.
      </p>
    </section>

    <section class="footer-section">
      <p class="version">Version 1.0</p>
    </section>
  </div>
</div>

<style>
  .about-container {
    min-height: 100vh;
    padding: 24px;
    background: var(--bg-primary);
  }

  .about-card {
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
    margin: 0 0 24px;
    font-size: 28px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .intro {
    font-size: 18px;
    color: var(--text-secondary);
    line-height: 1.6;
  }

  section {
    margin-bottom: 32px;
  }

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 16px;
  }

  p {
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 12px;
  }

  ul, ol {
    color: var(--text-secondary);
    line-height: 1.8;
    margin: 0;
    padding-left: 24px;
  }

  li {
    margin-bottom: 8px;
  }

  li strong {
    color: var(--text-primary);
  }

  .footer-section {
    margin-top: 48px;
    padding-top: 24px;
    border-top: 1px solid var(--border);
    text-align: center;
  }

  .version {
    color: var(--text-muted);
    font-size: 14px;
    margin: 0;
  }

  @media (max-width: 768px) {
    .about-card {
      padding: 24px;
    }

    h1 {
      font-size: 24px;
    }

    h2 {
      font-size: 18px;
    }

    .intro {
      font-size: 16px;
    }
  }
</style>
