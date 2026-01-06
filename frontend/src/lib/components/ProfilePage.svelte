<script lang="ts">
  import type { User } from '../stores';
  import type { Goal, Completion } from '../api';

  export let user: User | null;
  export let isGuest: boolean;
  export let goals: Goal[] = [];
  export let completions: Completion[] = [];
  export let onBack: () => void;

  function calculateStreaks(goalCompletions: Completion[]) {
    if (goalCompletions.length === 0) {
      return { currentStreak: 0, longestStreak: 0 };
    }

    // Sort by date ascending
    const sortedDates = goalCompletions
      .map(c => c.date)
      .sort();

    // Remove duplicates (in case of multiple entries on the same day)
    const uniqueDates = [...new Set(sortedDates)];

    // Calculate current streak (counting backward from today)
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const todayStr = today.toISOString().split('T')[0];

    let currentStreak = 0;
    let checkDate = new Date(today);

    // Check if today or yesterday has a completion to start the streak
    const lastCompletion = uniqueDates[uniqueDates.length - 1];
    const lastCompletionDate = new Date(lastCompletion + 'T00:00:00');
    const daysSinceLastCompletion = Math.floor((today.getTime() - lastCompletionDate.getTime()) / (1000 * 60 * 60 * 24));

    if (daysSinceLastCompletion <= 1) {
      // Start counting from the last completion date
      checkDate = new Date(lastCompletionDate);
      const dateSet = new Set(uniqueDates);

      while (dateSet.has(checkDate.toISOString().split('T')[0])) {
        currentStreak++;
        checkDate.setDate(checkDate.getDate() - 1);
      }
    }

    // Calculate longest streak
    let longestStreak = 0;
    let streak = 1;

    for (let i = 1; i < uniqueDates.length; i++) {
      const prevDate = new Date(uniqueDates[i - 1] + 'T00:00:00');
      const currDate = new Date(uniqueDates[i] + 'T00:00:00');
      const dayDiff = Math.floor((currDate.getTime() - prevDate.getTime()) / (1000 * 60 * 60 * 24));

      if (dayDiff === 1) {
        streak++;
      } else {
        longestStreak = Math.max(longestStreak, streak);
        streak = 1;
      }
    }
    longestStreak = Math.max(longestStreak, streak);

    return { currentStreak, longestStreak };
  }

  function calculateBestPeriod(goalCompletions: Completion[]) {
    if (goalCompletions.length === 0) {
      return { bestWeek: 0, bestMonth: 0 };
    }

    // Group by week
    const weekMap = new Map<string, number>();
    const monthMap = new Map<string, number>();

    goalCompletions.forEach(c => {
      const date = new Date(c.date);
      const weekKey = getISOWeek(date);
      const monthKey = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;

      weekMap.set(weekKey, (weekMap.get(weekKey) || 0) + 1);
      monthMap.set(monthKey, (monthMap.get(monthKey) || 0) + 1);
    });

    const bestWeek = Math.max(...Array.from(weekMap.values()), 0);
    const bestMonth = Math.max(...Array.from(monthMap.values()), 0);

    return { bestWeek, bestMonth };
  }

  function calculateStats(goal: Goal, completions: Completion[]) {
    const goalCompletions = completions.filter(c => c.goal_id === goal.id);
    const daysCompleted = goalCompletions.length;

    if (daysCompleted === 0) {
      return {
        daysCompleted,
        daysSinceFirstCompletion: 0,
        rate: 0,
        periodStats: null,
        currentStreak: 0,
        longestStreak: 0,
        bestWeek: 0,
        bestMonth: 0
      };
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

    // Calculate streaks
    const { currentStreak, longestStreak } = calculateStreaks(goalCompletions);

    // Calculate best week/month
    const { bestWeek, bestMonth } = calculateBestPeriod(goalCompletions);

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

    return { daysCompleted, daysSinceFirstCompletion, rate, periodStats, currentStreak, longestStreak, bestWeek, bestMonth };
  }

  // Calculate overall summary stats
  function calculateOverallStats(goals: Goal[], completions: Completion[]) {
    const activeGoals = goals.filter(g => !g.archived_at);
    if (activeGoals.length === 0 || completions.length === 0) {
      return {
        totalCompletions: 0,
        avgCompletionRate: 0,
        bestOverallStreak: 0,
        totalActiveGoals: 0
      };
    }

    const totalCompletions = completions.length;
    const totalActiveGoals = activeGoals.length;

    // Calculate average completion rate across all goals
    let totalRate = 0;
    let bestOverallStreak = 0;

    activeGoals.forEach(goal => {
      const stats = calculateStats(goal, completions);
      totalRate += stats.rate;
      bestOverallStreak = Math.max(bestOverallStreak, stats.longestStreak);
    });

    const avgCompletionRate = Math.round(totalRate / activeGoals.length);

    return { totalCompletions, avgCompletionRate, bestOverallStreak, totalActiveGoals };
  }

  $: overallStats = calculateOverallStats(goals ?? [], completions ?? []);

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
  $: memberSince = isGuest && goals?.length > 0
    ? goals.reduce((earliest, goal) => {
        const goalDate = new Date(goal.created_at);
        return goalDate < earliest ? goalDate : earliest;
      }, new Date(goals[0].created_at)).toISOString()
    : null;

  function exportData() {
    const exportObj = {
      exportedAt: new Date().toISOString(),
      goals: (goals ?? []).map(g => ({
        name: g.name,
        color: g.color,
        created_at: g.created_at,
        target_count: g.target_count,
        target_period: g.target_period,
        archived_at: g.archived_at,
      })),
      completions: (completions ?? []).map(c => ({
        goal_name: goals.find(g => g.id === c.goal_id)?.name || 'Unknown',
        date: c.date,
        created_at: c.created_at,
      })),
    };

    const blob = new Blob([JSON.stringify(exportObj, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `goal-tracker-export-${new Date().toISOString().split('T')[0]}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }
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

    {#if goals && goals.length > 0 && completions && completions.length > 0}
      <div class="overall-stats">
        <h2 class="stats-title">Overview</h2>
        <div class="stats-grid">
          <div class="stat-card">
            <span class="stat-value">{overallStats.totalCompletions}</span>
            <span class="stat-label">Total Completions</span>
          </div>
          <div class="stat-card">
            <span class="stat-value">{overallStats.avgCompletionRate}%</span>
            <span class="stat-label">Avg Completion Rate</span>
          </div>
          <div class="stat-card">
            <span class="stat-value">{overallStats.bestOverallStreak}</span>
            <span class="stat-label">Best Streak (days)</span>
          </div>
          <div class="stat-card">
            <span class="stat-value">{overallStats.totalActiveGoals}</span>
            <span class="stat-label">Active Goals</span>
          </div>
        </div>
      </div>

      <div class="divider"></div>
    {/if}

    <div class="stats-section">
      <h2 class="stats-title">Goal Statistics</h2>

      {#if !goals || goals.length === 0}
        <p class="no-goals">No goals yet. Create your first goal to start tracking!</p>
      {:else}
        <div class="goals-list">
          {#each (goals ?? []).filter(g => !g.archived_at) as goal (goal.id)}
            {@const stats = calculateStats(goal, completions)}
            <div class="goal-stat">
              <div class="goal-info">
                <span class="goal-dot" style="background-color: {goal.color}"></span>
                <span class="goal-name">{goal.name}</span>
              </div>
              <div class="goal-stats-row">
                <p class="goal-progress">
                  {stats.daysCompleted} days ({stats.rate}% rate)
                </p>
                {#if stats.currentStreak > 0}
                  <span class="streak-badge current">
                    {stats.currentStreak} day streak
                  </span>
                {/if}
              </div>
              <div class="goal-details">
                <span class="detail-item">
                  <span class="detail-icon">&#128293;</span>
                  Best: {stats.longestStreak} days
                </span>
                <span class="detail-item">
                  <span class="detail-icon">&#128197;</span>
                  Best week: {stats.bestWeek}
                </span>
                <span class="detail-item">
                  <span class="detail-icon">&#128198;</span>
                  Best month: {stats.bestMonth}
                </span>
              </div>
              {#if stats.periodStats}
                <p class="goal-period-success">
                  {stats.periodStats.successRate}% of {stats.periodStats.period} target met ({stats.periodStats.successful}/{stats.periodStats.total})
                </p>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="divider"></div>

    <div class="export-section">
      <h2 class="section-title">Data Export</h2>
      <p class="export-description">Download a copy of all your goals and completions.</p>
      <button class="export-button" on:click={exportData}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
          <path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z"/>
        </svg>
        Export Data (JSON)
      </button>
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
    max-width: 30rem;
    width: 100%;
    margin: 0 auto;
    padding: 1.5rem;
    box-sizing: border-box;
  }

  .back-button {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: 1rem;
    cursor: pointer;
    padding: 0.5rem 0;
    margin-bottom: 1.5rem;
  }

  .back-button:hover {
    color: var(--text-primary);
  }

  .profile-header {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 1rem 0;
  }

  .avatar {
    width: 5rem;
    height: 5rem;
    border-radius: 50%;
    background: var(--bg-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    margin-bottom: 1rem;
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
    font-size: 2rem;
    font-weight: 600;
    color: var(--text-primary);
    text-transform: uppercase;
  }

  .user-name {
    font-size: 1.5rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 0.25rem 0;
  }

  .user-email {
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin: 0 0 0.5rem 0;
  }

  .member-since {
    font-size: 0.8125rem;
    color: var(--text-muted, var(--text-secondary));
    margin: 0;
  }

  .divider {
    height: 1px;
    background: var(--border);
    margin: 1.5rem 0;
  }

  .stats-section {
    flex: 1;
  }

  .stats-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 1.25rem 0;
  }

  .no-goals {
    color: var(--text-secondary);
    font-size: 0.875rem;
    text-align: center;
    padding: 2rem 0;
  }

  .goals-list {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .goal-stat {
    padding: 0.75rem 0;
    border-bottom: 1px solid var(--border);
  }

  .goal-stat:last-child {
    border-bottom: none;
  }

  .goal-info {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    margin-bottom: 0.25rem;
  }

  .goal-dot {
    width: 0.75rem;
    height: 0.75rem;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .goal-name {
    font-size: 1rem;
    font-weight: 500;
    color: var(--text-primary);
  }

  .goal-progress {
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin: 0 0 0 1.375rem;
  }

  .goal-period-success {
    font-size: 0.8125rem;
    color: var(--accent);
    margin: 0.25rem 0 0 1.375rem;
    font-weight: 500;
  }

  /* Overall stats grid */
  .overall-stats {
    margin-bottom: 0.5rem;
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.75rem;
  }

  .stat-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
  }

  .stat-value {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--accent);
    line-height: 1.2;
  }

  .stat-label {
    font-size: 0.75rem;
    color: var(--text-secondary);
    margin-top: 0.25rem;
  }

  /* Goal stats row with streak badge */
  .goal-stats-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-left: 1.375rem;
    margin-bottom: 0.375rem;
  }

  .goal-stats-row .goal-progress {
    margin: 0;
  }

  .streak-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.125rem 0.5rem;
    border-radius: 0.75rem;
    font-size: 0.6875rem;
    font-weight: 600;
  }

  .streak-badge.current {
    background: var(--accent);
    color: white;
  }

  /* Goal details row */
  .goal-details {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
    margin-left: 1.375rem;
    margin-top: 0.25rem;
    margin-bottom: 0.25rem;
  }

  .detail-item {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  .detail-icon {
    font-size: 0.75rem;
  }

  @media (max-width: 480px) {
    .content {
      padding: 1rem;
    }

    .back-button {
      margin-bottom: 1rem;
    }

    .avatar {
      width: 4rem;
      height: 4rem;
    }

    .avatar.guest svg {
      width: 2rem;
      height: 2rem;
    }

    .avatar-initial {
      font-size: 1.75rem;
    }

    .user-name {
      font-size: 1.25rem;
    }

    .stats-title {
      font-size: 1rem;
    }

    .goal-name {
      font-size: 0.9375rem;
    }

    .goal-progress {
      font-size: 0.8125rem;
    }

    .section-title {
      font-size: 1rem;
    }
  }

  /* Export section styles */
  .export-section {
    padding: 0.5rem 0;
  }

  .section-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 0.5rem 0;
  }

  .export-description {
    font-size: 0.875rem;
    color: var(--text-secondary);
    margin: 0 0 1rem 0;
  }

  .export-button {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.625rem 1rem;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    font-size: 0.875rem;
    color: var(--text-primary);
    cursor: pointer;
    transition: background-color 0.15s;
  }

  .export-button:hover {
    background: var(--bg-tertiary);
  }
</style>
