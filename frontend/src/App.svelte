<script lang="ts">
  import { onMount } from 'svelte';
  import Header from './lib/components/Header.svelte';
  import Footer from './lib/components/Footer.svelte';
  import GoalRow from './lib/components/GoalRow.svelte';
  import GoalEditor from './lib/components/GoalEditor.svelte';
  import AuthPage from './lib/components/AuthPage.svelte';
  import ProfilePage from './lib/components/ProfilePage.svelte';
  import PrivacyPolicy from './lib/components/PrivacyPolicy.svelte';
  import {
    getCalendar,
    createGoal,
    updateGoal,
    archiveGoal,
    createCompletion,
    deleteCompletion,
    reorderGoals,
    getCurrentUser,
    logout,
    getAllCompletions,
    type Goal,
    type Completion,
  } from './lib/api';
  import { authStore, hasLocalData, setGuestMode, type AuthState } from './lib/stores';
  import { syncManager, syncStatus, type SyncStatus } from './lib/sync';

  // Color palette for auto-assigned goal colors (alternating green and slate gray)
  const GOAL_PALETTE = [
    '#5B8C5A', // Sage green
    '#708090', // Slate gray
  ];

  // Auth state
  let authState: AuthState;
  authStore.subscribe(value => authState = value);

  // Sync state
  let currentSyncStatus: SyncStatus;
  syncStatus.subscribe(value => currentSyncStatus = value);

  // Current month in YYYY-MM format
  let currentMonth = new Date().toISOString().slice(0, 7);
  let goals: Goal[] = [];
  let completions: Completion[] = [];
  let loading = true;
  let error = '';

  // Editor state: null = main view, { mode: 'add' } = add goal, { mode: 'edit', goal } = edit goal
  type EditorState = null | { mode: 'add' } | { mode: 'edit'; goal: Goal };
  let editorState: EditorState = null;

  // Profile state
  let showProfile = false;
  let allCompletions: Completion[] = [];

  // Route state for legal pages
  type Route = 'home' | 'privacy';
  let currentRoute: Route = 'home';

  function getRouteFromPath(): Route {
    const path = window.location.pathname;
    if (path === '/privacy') return 'privacy';
    return 'home';
  }

  function navigateTo(route: Route) {
    currentRoute = route;
    const path = route === 'home' ? '/' : `/${route}`;
    window.history.pushState({}, '', path);
  }

  function handlePopState() {
    currentRoute = getRouteFromPath();
  }

  // Derived user state
  $: user = authState.type === 'authenticated' ? authState.user : null;
  $: isGuest = authState.type === 'guest';

  // Drag & drop state
  let draggedGoalId: string | null = null;
  let dragOverGoalId: string | null = null;

  // Compute days in current month
  $: {
    const [year, month] = currentMonth.split('-').map(Number);
    daysInMonth = new Date(year, month, 0).getDate();
  }
  let daysInMonth: number;

  // Compute currentDay for disabling future dates
  // If viewing current month: currentDay = today's day number
  // If viewing past month: currentDay = 0 (no restriction)
  // If viewing future month: currentDay = 0 but all days disabled via different logic
  $: {
    const now = new Date();
    const todayMonth = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
    if (currentMonth === todayMonth) {
      // Current month: disable days after today
      currentDay = now.getDate();
    } else if (currentMonth < todayMonth) {
      // Past month: no restriction
      currentDay = 0;
    } else {
      // Future month: all days disabled (currentDay = -1 makes day > currentDay always true)
      currentDay = -1;
    }
  }
  let currentDay: number;

  // Map completions by goal
  $: completionsByGoal = completions.reduce((acc, c) => {
    const day = parseInt(c.date.split('-')[2], 10);
    if (!acc[c.goal_id]) acc[c.goal_id] = new Map();
    acc[c.goal_id].set(day, c.id);
    return acc;
  }, {} as Record<string, Map<number, string>>);

  // Auto-assign colors to goals based on their index
  $: goalsWithColors = goals.map((goal, index) => ({
    ...goal,
    color: GOAL_PALETTE[index % GOAL_PALETTE.length]
  }));

  // Reactive map of period completions per goal (updates when completions change)
  // Note: explicitly reference completions before reduce to ensure Svelte tracks it as a dependency
  $: periodCompletionsMap = ((allCompletions) => goals.reduce((acc, goal) => {
    if (!goal.target_period) {
      acc[goal.id] = 0;
      return acc;
    }

    const goalCompletions = allCompletions.filter(c => c.goal_id === goal.id);
    const now = new Date();

    if (goal.target_period === 'week') {
      // Get start of current week (Sunday)
      const dayOfWeek = now.getDay();
      const weekStart = new Date(now);
      weekStart.setDate(now.getDate() - dayOfWeek);
      weekStart.setHours(0, 0, 0, 0);

      acc[goal.id] = goalCompletions.filter(c => {
        const completionDate = new Date(c.date + 'T00:00:00');
        return completionDate >= weekStart && completionDate <= now;
      }).length;
    } else {
      // Current month
      const monthStart = new Date(now.getFullYear(), now.getMonth(), 1);
      acc[goal.id] = goalCompletions.filter(c => {
        const completionDate = new Date(c.date + 'T00:00:00');
        return completionDate >= monthStart && completionDate <= now;
      }).length;
    }
    return acc;
  }, {} as Record<string, number>))(completions);

  async function loadData() {
    loading = true;
    error = '';
    try {
      const data = await getCalendar(currentMonth);
      goals = data.goals;
      completions = data.completions;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load data';
    } finally {
      loading = false;
    }
  }

  function prevMonth() {
    const [year, month] = currentMonth.split('-').map(Number);
    const d = new Date(year, month - 2, 1);
    currentMonth = d.toISOString().slice(0, 7);
  }

  function nextMonth() {
    const [year, month] = currentMonth.split('-').map(Number);
    const d = new Date(year, month, 1);
    currentMonth = d.toISOString().slice(0, 7);
  }

  async function handleToggle(goalId: string, day: number) {
    const [year, month] = currentMonth.split('-');
    const date = `${year}-${month}-${day.toString().padStart(2, '0')}`;

    const goalCompletions = completionsByGoal[goalId];
    const existingId = goalCompletions?.get(day);

    try {
      if (existingId) {
        await deleteCompletion(existingId);
        completions = completions.filter(c => c.id !== existingId);
      } else {
        const newCompletion = await createCompletion(goalId, date);
        completions = [...completions, newCompletion];
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to update';
    }
  }

  async function handleEditorSave(data: { name: string; target_count?: number; target_period?: 'week' | 'month' }) {
    if (!editorState) return;

    try {
      if (editorState.mode === 'add') {
        // Color will be auto-assigned based on index, use placeholder for API
        const goal = await createGoal(
          data.name,
          GOAL_PALETTE[goals.length % GOAL_PALETTE.length],
          data.target_count,
          data.target_period
        );
        goals = [...goals, goal];
      } else {
        const updated = await updateGoal(editorState.goal.id, {
          name: data.name,
          target_count: data.target_count,
          target_period: data.target_period,
        });
        goals = goals.map(g => g.id === updated.id ? updated : g);
      }
      editorState = null;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save goal';
    }
  }

  async function handleEditorDelete() {
    if (!editorState || editorState.mode !== 'edit') return;

    try {
      await archiveGoal(editorState.goal.id);
      goals = goals.filter(g => g.id !== editorState!.goal.id);
      editorState = null;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to delete goal';
    }
  }

  function handleEditGoal(goal: Goal) {
    editorState = { mode: 'edit', goal };
  }


  // Drag & drop handlers
  function handleDragStart(goalId: string, e: DragEvent) {
    draggedGoalId = goalId;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', goalId);
    }
  }

  function handleDragOver(goalId: string, e: DragEvent) {
    e.preventDefault();
    if (draggedGoalId && draggedGoalId !== goalId) {
      dragOverGoalId = goalId;
    }
  }

  async function handleDrop(targetGoalId: string, e: DragEvent) {
    e.preventDefault();
    dragOverGoalId = null;

    if (!draggedGoalId || draggedGoalId === targetGoalId) {
      draggedGoalId = null;
      return;
    }

    // Reorder locally first for instant feedback
    const draggedIndex = goals.findIndex(g => g.id === draggedGoalId);
    const targetIndex = goals.findIndex(g => g.id === targetGoalId);

    if (draggedIndex === -1 || targetIndex === -1) {
      draggedGoalId = null;
      return;
    }

    const newGoals = [...goals];
    const [removed] = newGoals.splice(draggedIndex, 1);
    newGoals.splice(targetIndex, 0, removed);
    goals = newGoals;

    // Save to server
    try {
      await reorderGoals(goals.map(g => g.id));
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to reorder';
      // Reload to restore correct order
      await loadData();
    }

    draggedGoalId = null;
  }

  async function checkAuth() {
    // Initialize sync manager
    await syncManager.init();

    // Check if we have a session
    try {
      const user = await getCurrentUser();
      if (user) {
        authStore.set({ type: 'authenticated', user });
        // Sync local data with server on successful auth
        try {
          await syncManager.sync();
        } catch (syncError) {
          console.error('Initial sync failed:', syncError);
        }
        return;
      }
    } catch (e) {
      // Not authenticated
    }

    // Check if we have local data (guest mode)
    if (hasLocalData()) {
      authStore.set({ type: 'guest' });
      return;
    }

    // Not authenticated and no local data
    authStore.set({ type: 'unauthenticated' });
  }

  function handleContinueAsGuest() {
    setGuestMode(true);
    authStore.set({ type: 'guest' });
  }

  async function handleLogout() {
    try {
      await logout();
      setGuestMode(false);
      authStore.set({ type: 'unauthenticated' });
      goals = [];
      completions = [];
      allCompletions = [];
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to log out';
    }
  }

  async function handleProfileClick() {
    // Load all completions for statistics
    try {
      allCompletions = await getAllCompletions();
    } catch (e) {
      console.error('Failed to load completions for stats:', e);
      // Fall back to current month completions
      allCompletions = completions;
    }
    showProfile = true;
  }

  function handleProfileBack() {
    showProfile = false;
  }

  function handleSignIn() {
    // Redirect to Google OAuth
    const apiBase = typeof window !== 'undefined' && window.location.hostname !== 'localhost'
      ? '/api/v1'
      : 'http://localhost:8080/api/v1';
    window.location.href = `${apiBase}/auth/google`;
  }

  async function handleSyncRetry() {
    await syncManager.retry();
  }

  function handleSyncDismiss() {
    syncManager.dismissError();
  }

  onMount(async () => {
    // Initialize route from URL
    currentRoute = getRouteFromPath();
    window.addEventListener('popstate', handlePopState);

    await checkAuth();
    // Only load data if authenticated or guest
    if (authState.type === 'authenticated' || authState.type === 'guest') {
      await loadData();
    }

    return () => {
      window.removeEventListener('popstate', handlePopState);
    };
  });

  // Reload data when month changes, but only if authenticated/guest
  $: if (authState.type === 'authenticated' || authState.type === 'guest') {
    currentMonth, loadData();
  }
</script>

{#if currentRoute === 'privacy'}
  <PrivacyPolicy onBack={() => navigateTo('home')} />
{:else if authState.type === 'loading'}
  <div class="loading-container">
    <p class="loading">Loading...</p>
  </div>
{:else if authState.type === 'unauthenticated'}
  <AuthPage onContinueAsGuest={handleContinueAsGuest} />
{:else}
  <div class="app-container">
    {#if showProfile}
      <ProfilePage
        {user}
        {isGuest}
        {goals}
        completions={allCompletions}
        onBack={handleProfileBack}
      />
    {:else if editorState}
      <GoalEditor
        mode={editorState.mode}
        goal={editorState.mode === 'edit' ? goalsWithColors.find(g => g.id === editorState.goal.id) ?? null : null}
        previewColor={GOAL_PALETTE[goals.length % GOAL_PALETTE.length]}
        onSave={handleEditorSave}
        onCancel={() => editorState = null}
        onDelete={editorState.mode === 'edit' ? handleEditorDelete : null}
      />
    {:else}
      <Header
        month={currentMonth}
        onPrev={prevMonth}
        onNext={nextMonth}
        showAddForm={false}
        onToggleAddForm={() => editorState = { mode: 'add' }}
        {user}
        {isGuest}
        onLogout={handleLogout}
        onProfileClick={handleProfileClick}
        onSignIn={handleSignIn}
      />

      <main>
        {#if error}
          <div class="error">{error}</div>
        {/if}

        {#if currentSyncStatus.state === 'syncing'}
          <div class="sync-banner sync-syncing">
            <span class="sync-spinner"></span>
            <span>{currentSyncStatus.message}</span>
          </div>
        {:else if currentSyncStatus.state === 'success'}
          <div class="sync-banner sync-success">
            <span>{currentSyncStatus.message}</span>
          </div>
        {:else if currentSyncStatus.state === 'error'}
          <div class="sync-banner sync-error">
            <span>{currentSyncStatus.message}</span>
            <div class="sync-actions">
              {#if currentSyncStatus.canRetry}
                <button class="sync-btn" on:click={handleSyncRetry}>Retry</button>
              {/if}
              <button class="sync-btn sync-btn-dismiss" on:click={handleSyncDismiss}>Dismiss</button>
            </div>
          </div>
        {/if}

      {#if loading}
        <p class="loading">Loading...</p>
      {:else if goals.length === 0}
        <p class="empty">No goals yet. Add one to get started!</p>
      {:else}
        <div class="goals" role="list">
          {#each goalsWithColors as goal (goal.id)}
            <GoalRow
              {goal}
              {daysInMonth}
              {currentDay}
              month={currentMonth}
              completedDays={completionsByGoal[goal.id] ? new Set(completionsByGoal[goal.id].keys()) : new Set()}
              periodCompletions={periodCompletionsMap[goal.id] ?? 0}
              onToggle={(day) => handleToggle(goal.id, day)}
              onEdit={() => handleEditGoal(goal)}
              onDragStart={(e) => handleDragStart(goal.id, e)}
              onDragOver={(e) => handleDragOver(goal.id, e)}
              onDrop={(e) => handleDrop(goal.id, e)}
              isDragOver={dragOverGoalId === goal.id}
            />
          {/each}
        </div>
      {/if}
      </main>

      <Footer />
    {/if}
  </div>
{/if}

<style>
  :global(:root) {
    /* Backgrounds - warm yellow tint */
    --bg-primary: #F7F3E3;      /* Main background - warm yellow */
    --bg-secondary: #EFE9D8;    /* Secondary - slightly darker */
    --bg-tertiary: #E5DECA;     /* Tertiary - warm beige */

    /* Text - dark for contrast */
    --text-primary: #2D2A26;    /* Main text - warm dark */
    --text-secondary: #6B6560;  /* Secondary text */
    --text-muted: #9A948C;      /* Muted text */

    /* Accent - sage green (primary color) */
    --accent: #5B8C5A;
    --accent-hover: #4A7349;

    /* Status colors */
    --success: #5B8C5A;         /* Sage green */
    --error: #C65D4A;           /* Muted red */
    --error-bg: #FEF2F0;        /* Light red bg */

    /* Border */
    --border: #D5CEBC;          /* Warm gray border */
  }

  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
    margin: 0;
    padding: 0;
    background: var(--bg-primary);
    color: var(--text-primary);
  }

  .app-container {
    display: grid;
    grid-template-rows: auto 1fr auto;
    min-height: 100vh;
  }

  main {
    padding: 24px 0;
    width: 100%;
    box-sizing: border-box;
  }

  .error {
    padding: 12px;
    margin: 0 24px 16px;
    background: var(--error-bg);
    color: var(--error);
    border-radius: 4px;
    border: 1px solid var(--error);
  }

  .loading, .empty {
    color: var(--text-secondary);
    font-style: italic;
    text-align: center;
    padding: 0 24px;
  }

  .goals {
    width: 100%;
  }

  .loading-container {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-primary);
  }

  /* Sync status banner */
  .sync-banner {
    padding: 12px 16px;
    margin: 0 24px 16px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 14px;
  }

  .sync-syncing {
    background: var(--bg-secondary);
    color: var(--text-secondary);
    border: 1px solid var(--border);
  }

  .sync-success {
    background: #E8F5E9;
    color: var(--success);
    border: 1px solid var(--success);
  }

  .sync-error {
    background: var(--error-bg);
    color: var(--error);
    border: 1px solid var(--error);
    flex-wrap: wrap;
  }

  .sync-spinner {
    width: 16px;
    height: 16px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .sync-actions {
    display: flex;
    gap: 8px;
    margin-left: auto;
  }

  .sync-btn {
    padding: 4px 12px;
    border-radius: 4px;
    border: none;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
  }

  .sync-btn:not(.sync-btn-dismiss) {
    background: var(--accent);
    color: white;
  }

  .sync-btn:not(.sync-btn-dismiss):hover {
    background: var(--accent-hover);
  }

  .sync-btn-dismiss {
    background: transparent;
    color: var(--error);
    text-decoration: underline;
  }

  .sync-btn-dismiss:hover {
    background: rgba(0, 0, 0, 0.05);
  }
</style>
