<script lang="ts">
  import { onMount } from 'svelte';
  import Header from './lib/components/Header.svelte';
  import Footer from './lib/components/Footer.svelte';
  import GoalRow from './lib/components/GoalRow.svelte';
  import GoalEditor from './lib/components/GoalEditor.svelte';
  import AuthPage from './lib/components/AuthPage.svelte';
  import ProfilePage from './lib/components/ProfilePage.svelte';
  import PrivacyPolicy from './lib/components/PrivacyPolicy.svelte';
  import Spinner from './lib/components/Spinner.svelte';
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
    getCurrentPeriodCompletions,
    type Goal,
    type Completion,
  } from './lib/api';
  import { getUserFriendlyMessage } from './lib/errors';
  import { authStore, hasLocalData, setGuestMode, isOnline, type AuthState } from './lib/stores';
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

  // Online/offline state
  let online = true;
  isOnline.subscribe(value => online = value);

  // Current month in YYYY-MM format
  let currentMonth = new Date().toISOString().slice(0, 7);
  let goals: Goal[] = [];
  let completions: Completion[] = [];
  let periodCompletions: Completion[] = [];
  let loading = true;
  let error = '';

  // Editor state: null = main view, { mode: 'add' } = add goal, { mode: 'edit', goal } = edit goal
  type EditorState = null | { mode: 'add' } | { mode: 'edit'; goal: Goal };
  let editorState: EditorState = null;

  // Profile state
  let showProfile = false;
  let allCompletions: Completion[] = [];

  // Flag to prevent race condition between sync and loadData during initial auth
  let initialAuthInProgress = false;

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
  $: completionsByGoal = (completions ?? []).reduce((acc, c) => {
    const day = parseInt(c.date.split('-')[2], 10);
    if (!acc[c.goal_id]) acc[c.goal_id] = new Map();
    acc[c.goal_id].set(day, c.id);
    return acc;
  }, {} as Record<string, Map<number, string>>);

  // Auto-assign colors to goals based on their index
  $: goalsWithColors = (goals ?? []).map((goal, index) => ({
    ...goal,
    color: GOAL_PALETTE[index % GOAL_PALETTE.length]
  }));

  // Reactive map of period completions per goal (uses periodCompletions for accurate weekly/monthly counts)
  // Note: Pass goals as a second IIFE parameter to ensure Svelte detects it as a reactive dependency
  $: periodCompletionsMap = ((allCompletions, allGoals) => {
    return allGoals.reduce((acc, goal) => {
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
    }, {} as Record<string, number>);
  })(periodCompletions ?? [], goals ?? []);

  async function loadData() {
    loading = true;
    error = '';
    try {
      // Fetch both calendar data and period completions before setting state
      // This prevents the periodCompletionsMap reactive from running with stale data
      const [data, periodData] = await Promise.all([
        getCalendar(currentMonth),
        getCurrentPeriodCompletions()
      ]);

      // Set periodCompletions FIRST so the reactive has data when goals triggers it
      periodCompletions = periodData;
      goals = data.goals ?? [];
      completions = data.completions ?? [];
    } catch (e) {
      console.error('[loadData] error:', e);
      error = getUserFriendlyMessage(e);
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

  // Touch swipe handling for month navigation
  let touchStartX = 0;
  let touchStartY = 0;
  const SWIPE_THRESHOLD = 50; // Minimum swipe distance in pixels
  const SWIPE_RATIO = 1.5; // Horizontal must be 1.5x vertical to count as horizontal swipe

  function handleTouchStart(e: TouchEvent) {
    touchStartX = e.touches[0].clientX;
    touchStartY = e.touches[0].clientY;
  }

  function handleTouchEnd(e: TouchEvent) {
    const touchEndX = e.changedTouches[0].clientX;
    const touchEndY = e.changedTouches[0].clientY;
    const deltaX = touchEndX - touchStartX;
    const deltaY = touchEndY - touchStartY;

    // Check if horizontal swipe is significant and dominant
    if (Math.abs(deltaX) > SWIPE_THRESHOLD && Math.abs(deltaX) > Math.abs(deltaY) * SWIPE_RATIO) {
      if (deltaX > 0) {
        prevMonth(); // Swipe right = previous month
      } else {
        nextMonth(); // Swipe left = next month
      }
    }
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
        periodCompletions = periodCompletions.filter(c => c.id !== existingId);
      } else {
        const newCompletion = await createCompletion(goalId, date);
        completions = [...completions, newCompletion];
        periodCompletions = [...periodCompletions, newCompletion];
      }
    } catch (e) {
      error = getUserFriendlyMessage(e);
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
      error = getUserFriendlyMessage(e);
    }
  }

  async function handleEditorDelete() {
    if (!editorState || editorState.mode !== 'edit') return;

    try {
      await archiveGoal(editorState.goal.id);
      goals = goals.filter(g => g.id !== editorState!.goal.id);
      editorState = null;
    } catch (e) {
      error = getUserFriendlyMessage(e);
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
      error = getUserFriendlyMessage(e);
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
        // Set flag to prevent reactive from triggering loadData during sync
        initialAuthInProgress = true;
        authStore.set({ type: 'authenticated', user });
        // Sync local data with server on successful auth
        try {
          await syncManager.sync();
        } catch (syncError) {
          console.error('Initial sync failed:', syncError);
        }
        // Now explicitly load data after sync completes
        initialAuthInProgress = false;
        await loadData();
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
      error = getUserFriendlyMessage(e);
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

  // Keyboard navigation state
  let focusedGoalIndex = -1;

  function handleKeyDown(e: KeyboardEvent) {
    // Ignore shortcuts when typing in input fields
    const target = e.target as HTMLElement;
    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
      // Only handle Escape in input fields
      if (e.key === 'Escape') {
        (target as HTMLInputElement).blur();
      }
      return;
    }

    // Don't handle shortcuts on non-main views
    if (currentRoute !== 'home' || authState.type !== 'authenticated' && authState.type !== 'guest') {
      return;
    }

    // Handle Escape to close modals/forms and profile
    if (e.key === 'Escape') {
      if (showProfile) {
        showProfile = false;
        e.preventDefault();
        return;
      }
      if (editorState) {
        editorState = null;
        e.preventDefault();
        return;
      }
      // Reset goal focus
      focusedGoalIndex = -1;
      return;
    }

    // Don't handle other shortcuts when in editor/profile
    if (editorState || showProfile) {
      return;
    }

    // Arrow keys for month navigation
    if (e.key === 'ArrowLeft') {
      prevMonth();
      e.preventDefault();
      return;
    }
    if (e.key === 'ArrowRight') {
      nextMonth();
      e.preventDefault();
      return;
    }

    // 'N' to open new goal form
    if (e.key === 'n' || e.key === 'N') {
      editorState = { mode: 'add' };
      e.preventDefault();
      return;
    }

    // Arrow up/down for goal navigation
    if (e.key === 'ArrowUp' && goals.length > 0) {
      focusedGoalIndex = focusedGoalIndex <= 0 ? goals.length - 1 : focusedGoalIndex - 1;
      e.preventDefault();
      return;
    }
    if (e.key === 'ArrowDown' && goals.length > 0) {
      focusedGoalIndex = focusedGoalIndex >= goals.length - 1 ? 0 : focusedGoalIndex + 1;
      e.preventDefault();
      return;
    }

    // Enter to edit focused goal
    if (e.key === 'Enter' && focusedGoalIndex >= 0 && focusedGoalIndex < goals.length) {
      handleEditGoal(goals[focusedGoalIndex]);
      e.preventDefault();
      return;
    }

    // Number keys 1-9 and 0 (for 10) plus Shift+1-9 for 11-19, etc. to toggle today's completion
    // Simple approach: 1-9 maps to days 1-9, 0 maps to day 10
    // For days 11-31, use combinations or just allow the basic 1-9, 0
    if (focusedGoalIndex >= 0 && focusedGoalIndex < goals.length) {
      const key = e.key;
      let day: number | null = null;

      // Check for number keys
      if (key >= '1' && key <= '9') {
        if (e.shiftKey) {
          // Shift + 1-9 for days 21-29
          day = 20 + parseInt(key);
        } else if (e.altKey) {
          // Alt + 1-9 for days 11-19 (but alt often has browser shortcuts, so check ctrlKey too)
          day = 10 + parseInt(key);
        } else if (e.ctrlKey) {
          // Ctrl + 1 for 31
          if (key === '1') day = 31;
          else day = 30 + parseInt(key); // Won't really be used but for completeness
        } else {
          day = parseInt(key);
        }
      } else if (key === '0') {
        if (e.shiftKey) {
          day = 30;
        } else if (e.altKey) {
          day = 20;
        } else {
          day = 10;
        }
      }

      if (day !== null && day >= 1 && day <= daysInMonth) {
        // Check if day is in the past or today (not future)
        if (currentDay === 0 || (currentDay > 0 && day <= currentDay)) {
          handleToggle(goals[focusedGoalIndex].id, day);
          e.preventDefault();
        }
      }
    }
  }

  onMount(async () => {
    // Initialize route from URL
    currentRoute = getRouteFromPath();
    window.addEventListener('popstate', handlePopState);
    window.addEventListener('keydown', handleKeyDown);

    await checkAuth();
    // Note: loadData() is called by the reactive statement when authState changes,
    // so we don't need to call it here explicitly

    return () => {
      window.removeEventListener('popstate', handlePopState);
      window.removeEventListener('keydown', handleKeyDown);
    };
  });

  // Reload data when month changes, but only if authenticated/guest
  // Skip during initial auth to avoid race condition with sync
  $: if ((authState.type === 'authenticated' || authState.type === 'guest') && !initialAuthInProgress) {
    currentMonth, loadData();
  }
</script>

{#if currentRoute === 'privacy'}
  <PrivacyPolicy onBack={() => navigateTo('home')} />
{:else if authState.type === 'loading'}
  <div class="loading-container">
    <Spinner size="large" />
    <p class="loading-text">Loading your goals...</p>
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

      <main on:touchstart={handleTouchStart} on:touchend={handleTouchEnd}>
        {#if error}
          <div class="error" role="alert" aria-live="assertive">{error}</div>
        {/if}

        {#if currentSyncStatus.state === 'syncing'}
          <div class="sync-banner sync-syncing" role="status" aria-live="polite">
            <span class="sync-spinner" aria-hidden="true"></span>
            <span>{currentSyncStatus.message}</span>
          </div>
        {/if}

        {#if !online}
          <div class="offline-banner" role="alert" aria-live="assertive">
            <span class="offline-icon" aria-hidden="true">⚡</span>
            <span>You're offline. Changes will be saved locally.</span>
          </div>
        {/if}

      {#if loading}
        <div class="inline-loading">
          <Spinner size="medium" />
        </div>
      {:else if goals.length === 0}
        <div class="welcome-card">
          <h2 class="welcome-title">Welcome to Goal Tracker!</h2>
          <p class="welcome-text">Track daily habits and goals with a visual calendar.</p>
          <ul class="welcome-features">
            <li><strong>Create goals</strong> - Click "New Goal" to start tracking</li>
            <li><strong>Mark completions</strong> - Click day squares to toggle</li>
            <li><strong>Set targets</strong> - Optional weekly/monthly targets with progress bars</li>
            <li><strong>Swipe to navigate</strong> - View past or future months</li>
          </ul>
          <button class="welcome-cta" on:click={() => editorState = { mode: 'add' }}>
            Create Your First Goal
          </button>
        </div>
      {:else}
        <div class="goals" role="list">
          {#each goalsWithColors as goal, index (goal.id)}
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
              isFocused={focusedGoalIndex === index}
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

    /* Text - dark for contrast (WCAG AA compliant) */
    --text-primary: #2D2A26;    /* Main text - warm dark */
    --text-secondary: #5D5853;  /* Secondary text - 4.8:1 contrast */
    --text-muted: #736D65;      /* Muted text - 4.6:1 contrast */

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
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    background: var(--bg-primary);
  }

  .loading-text {
    color: var(--text-muted);
    font-size: 14px;
  }

  .inline-loading {
    display: flex;
    justify-content: center;
    padding: 48px 0;
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

  /* Offline indicator banner */
  .offline-banner {
    padding: 12px 16px;
    margin: 0 24px 16px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    background: #FFF3CD;
    color: #856404;
    border: 1px solid #FFEEBA;
  }

  .offline-icon {
    font-size: 16px;
  }

  /* Welcome/onboarding card */
  .welcome-card {
    max-width: 400px;
    margin: 32px auto;
    padding: 24px;
    background: var(--bg-secondary);
    border-radius: 8px;
    text-align: center;
  }

  .welcome-title {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 8px 0;
  }

  .welcome-text {
    font-size: 14px;
    color: var(--text-secondary);
    margin: 0 0 20px 0;
  }

  .welcome-features {
    text-align: left;
    padding-left: 20px;
    margin: 0 0 24px 0;
    font-size: 14px;
    color: var(--text-secondary);
    line-height: 1.8;
  }

  .welcome-features li {
    margin-bottom: 4px;
  }

  .welcome-features strong {
    color: var(--text-primary);
  }

  .welcome-cta {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 10px 20px;
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 6px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: background-color 0.15s;
  }

  .welcome-cta:hover {
    background: var(--accent-hover);
  }
</style>
