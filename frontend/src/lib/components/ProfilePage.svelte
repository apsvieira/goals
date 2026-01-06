<script lang="ts">
  import type { User } from '../stores';
  import type { Goal, Completion } from '../api';

  export let user: User | null;
  export let isGuest: boolean;
  export let goals: Goal[];
  export let completions: Completion[];
  export let onBack: () => void;

  function calculateStats(goal: Goal, completions: Completion[]) {
    const goalCompletions = completions.filter(c => c.goal_id === goal.id);
    const daysCompleted = goalCompletions.length;

    if (daysCompleted === 0) {
      return { daysCompleted, daysSinceFirstCompletion: 0, rate: 0, periodStats: null };
    }

    // Find the earliest completion date for this goal
    const sortedDates = goalCompletions
      .map(c => new Date(c.date))
      .sort((a, b) => a.getTime() - b.getTime());
    const firstCompletionDate = sortedDates[0];

    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const daysSinceFirstCompletion = Math.floor((today.getTime() - firstCompletionDate.getTime()) / (1000 * 60 * 60 * 24)) + 1;

    const rate = daysSinceFirstCompletion > 0 ? Math.round((daysCompleted / daysSinceFirstCompletion) * 100) : 0;

    // Calculate weekly/monthly target success rate if goal has targets
    let periodStats = null;
    if (goal.target_count && goal.target_period) {
      const completionDates = goalCompletions.map(c => new Date(c.date));

      if (goal.target_period === 'week') {
        // Group completions by ISO week
        const weekMap = new Map<string, number>();
        completionDates.forEach(date => {
          const weekKey = getISOWeek(date);
          weekMap.set(weekKey, (weekMap.get(weekKey) || 0) + 1);
        });

        // Count weeks from first completion to now
        const totalWeeks = Math.ceil(daysSinceFirstCompletion / 7);
        const successfulWeeks = Array.from(weekMap.values()).filter(count => count >= goal.target_count!).length;
        const successRate = totalWeeks > 0 ? Math.round((successfulWeeks / totalWeeks) * 100) : 0;

        periodStats = { successRate, successful: successfulWeeks, total: totalWeeks, period: 'weeks' };
      } else if (goal.target_period === 'month') {
        // Group completions by month
        const monthMap = new Map<string, number>();
        completionDates.forEach(date => {
          const monthKey = `${date.getFullYear()}-${date.getMonth()}`;
          monthMap.set(monthKey, (monthMap.get(monthKey) || 0) + 1);
        });

        // Count months from first completion to now
        const firstMonth = new Date(firstCompletionDate.getFullYear(), firstCompletionDate.getMonth(), 1);
        const currentMonth = new Date(today.getFullYear(), today.getMonth(), 1);
        const totalMonths = Math.max(1, Math.round((currentMonth.getTime() - firstMonth.getTime()) / (1000 * 60 * 60 * 24 * 30)) + 1);
        const successfulMonths = Array.from(monthMap.values()).filter(count => count >= goal.target_count!).length;
        const successRate = totalMonths > 0 ? Math.round((successfulMonths / totalMonths) * 100) : 0;

        periodStats = { successRate, successful: successfulMonths, total: totalMonths, period: 'months' };
      }
    }

    return { daysCompleted, daysSinceFirstCompletion, rate, periodStats };
  }

  // Helper to get ISO week string
  function getISOWeek(date: Date): string {
    const d = new Date(date);
    d.setHours(0, 0, 0, 0);
    d.setDate(d.getDate() + 4 - (d.getDay() || 7));
    const yearStart = new Date(d.getFullYear(), 0, 1);
    const weekNo = Math.ceil((((d.getTime() - yearStart.getTime()) / 86400000) + 1) / 7);
    return `${d.getFullYear()}-W${weekNo}`;
  }

  function formatMemberSince(dateStr: string | undefined): string {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    return date.toLocaleDateString('en-US', { day: 'numeric', month: 'short', year: 'numeric' });
  }

  // Find the earliest goal creation date as a proxy for member since date for guests
  $: memberSince = isGuest && goals.length > 0
    ? goals.reduce((earliest, goal) => {
        const goalDate = new Date(goal.created_at);
        return goalDate < earliest ? goalDate : earliest;
      }, new Date(goals[0].created_at)).toISOString()
    : null;
</script>

<div class="profile-page">
  <div class="content">
    <button class="back-button" on:click={onBack}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
        <path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z"/>
      </svg>
      Back
    </button>

    <div class="profile-header">
      <div class="avatar" class:guest={isGuest}>
        {#if isGuest}
          <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
            <circle cx="12" cy="7" r="4"/>
          </svg>
        {:else if user?.avatar_url}
          <img src={user.avatar_url} alt="User avatar" />
        {:else}
          <span class="avatar-initial">{user?.name?.[0] || user?.email?.[0] || '?'}</span>
        {/if}
      </div>

      <h1 class="user-name">{isGuest ? 'Anonymous' : (user?.name || user?.email?.split('@')[0] || 'User')}</h1>

      {#if !isGuest && user?.email}
        <p class="user-email">{user.email}</p>
      {/if}

      {#if !isGuest && user?.created_at}
        <p class="member-since">Member since {formatMemberSince(user.created_at)}</p>
      {/if}
    </div>

    <div class="divider"></div>

    <div class="stats-section">
      <h2 class="stats-title">Goal Statistics</h2>

      {#if goals.length === 0}
        <p class="no-goals">No goals yet. Create your first goal to start tracking!</p>
      {:else}
        <div class="goals-list">
          {#each goals.filter(g => !g.archived_at) as goal (goal.id)}
            {@const stats = calculateStats(goal, completions)}
            <div class="goal-stat">
              <div class="goal-info">
                <span class="goal-dot" style="background-color: {goal.color}"></span>
                <span class="goal-name">{goal.name}</span>
              </div>
              <p class="goal-progress">
                {stats.daysCompleted} days completed ({stats.daysCompleted}/{stats.daysSinceFirstCompletion})
              </p>
              {#if stats.periodStats}
                <p class="goal-period-success">
                  {stats.periodStats.successRate}% of {stats.periodStats.period} succeeded ({stats.periodStats.successful}/{stats.periodStats.total})
                </p>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .profile-page {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: var(--bg-primary);
    z-index: 100;
    overflow-y: auto;
  }

  .content {
    max-width: 480px;
    width: 100%;
    margin: 0 auto;
    padding: 24px;
    box-sizing: border-box;
  }

  .back-button {
    display: flex;
    align-items: center;
    gap: 8px;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: 16px;
    cursor: pointer;
    padding: 8px 0;
    margin-bottom: 24px;
  }

  .back-button:hover {
    color: var(--text-primary);
  }

  .profile-header {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 16px 0;
  }

  .avatar {
    width: 80px;
    height: 80px;
    border-radius: 50%;
    background: var(--bg-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    margin-bottom: 16px;
    border: 2px solid var(--border);
  }

  .avatar.guest {
    background: var(--bg-tertiary);
    color: var(--text-secondary);
  }

  .avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .avatar-initial {
    font-size: 32px;
    font-weight: 600;
    color: var(--text-primary);
    text-transform: uppercase;
  }

  .user-name {
    font-size: 24px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 4px 0;
  }

  .user-email {
    font-size: 14px;
    color: var(--text-secondary);
    margin: 0 0 8px 0;
  }

  .member-since {
    font-size: 13px;
    color: var(--text-muted, var(--text-secondary));
    margin: 0;
  }

  .divider {
    height: 1px;
    background: var(--border);
    margin: 24px 0;
  }

  .stats-section {
    flex: 1;
  }

  .stats-title {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 20px 0;
  }

  .no-goals {
    color: var(--text-secondary);
    font-size: 14px;
    text-align: center;
    padding: 32px 0;
  }

  .goals-list {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .goal-stat {
    padding: 12px 0;
    border-bottom: 1px solid var(--border);
  }

  .goal-stat:last-child {
    border-bottom: none;
  }

  .goal-info {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 4px;
  }

  .goal-dot {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .goal-name {
    font-size: 16px;
    font-weight: 500;
    color: var(--text-primary);
  }

  .goal-progress {
    font-size: 14px;
    color: var(--text-secondary);
    margin: 0 0 0 22px;
  }

  .goal-period-success {
    font-size: 13px;
    color: var(--accent);
    margin: 4px 0 0 22px;
    font-weight: 500;
  }

  @media (max-width: 480px) {
    .content {
      padding: 16px;
    }

    .back-button {
      margin-bottom: 16px;
    }

    .avatar {
      width: 64px;
      height: 64px;
    }

    .avatar.guest svg {
      width: 32px;
      height: 32px;
    }

    .avatar-initial {
      font-size: 28px;
    }

    .user-name {
      font-size: 20px;
    }

    .stats-title {
      font-size: 16px;
    }

    .goal-name {
      font-size: 15px;
    }

    .goal-progress {
      font-size: 13px;
    }
  }
</style>
