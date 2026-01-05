<script lang="ts">
  import { onMount } from 'svelte';
  import Header from './lib/components/Header.svelte';
  import Footer from './lib/components/Footer.svelte';
  import GoalRow from './lib/components/GoalRow.svelte';
  import AddGoalForm from './lib/components/AddGoalForm.svelte';
  import {
    getCalendar,
    createGoal,
    updateGoal,
    archiveGoal,
    createCompletion,
    deleteCompletion,
    reorderGoals,
    type Goal,
    type Completion,
  } from './lib/api';
  import EditGoalModal from './lib/components/EditGoalModal.svelte';

  // Current month in YYYY-MM format
  let currentMonth = new Date().toISOString().slice(0, 7);
  let goals: Goal[] = [];
  let completions: Completion[] = [];
  let loading = true;
  let error = '';
  let showAddForm = false;
  let editingGoal: Goal | null = null;

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

  async function handleAddGoal(name: string, color: string) {
    try {
      const goal = await createGoal(name, color);
      goals = [...goals, goal];
      showAddForm = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to create goal';
    }
  }

  function handleEditGoal(goal: Goal) {
    editingGoal = goal;
  }

  async function handleSaveGoal(updates: { name?: string; color?: string }) {
    if (!editingGoal) return;

    try {
      const updated = await updateGoal(editingGoal.id, updates);
      goals = goals.map(g => g.id === updated.id ? updated : g);
      editingGoal = null;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to update goal';
    }
  }

  async function handleDeleteGoal() {
    if (!editingGoal) return;

    try {
      await archiveGoal(editingGoal.id);
      goals = goals.filter(g => g.id !== editingGoal!.id);
      editingGoal = null;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to delete goal';
    }
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

  onMount(loadData);
  $: currentMonth, loadData();
</script>

<div class="app-container">
  <Header
    month={currentMonth}
    onPrev={prevMonth}
    onNext={nextMonth}
    {showAddForm}
    onToggleAddForm={() => showAddForm = !showAddForm}
  />

  <main>
    {#if error}
      <div class="error">{error}</div>
    {/if}

    {#if showAddForm}
      <div class="form-container">
        <AddGoalForm
          onAdd={handleAddGoal}
          onCancel={() => showAddForm = false}
        />
      </div>
    {/if}

    {#if editingGoal}
      <EditGoalModal
        goal={editingGoal}
        onSave={handleSaveGoal}
        onDelete={handleDeleteGoal}
        onClose={() => editingGoal = null}
      />
    {/if}

    {#if loading}
      <p class="loading">Loading...</p>
    {:else if goals.length === 0}
      <p class="empty">No goals yet. Add one to get started!</p>
    {:else}
      <div class="goals" role="list">
        {#each goals as goal (goal.id)}
          <GoalRow
            {goal}
            {daysInMonth}
            {currentDay}
            completedDays={completionsByGoal[goal.id] ? new Set(completionsByGoal[goal.id].keys()) : new Set()}
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
</div>

<style>
  :global(:root) {
    --bg-primary: #0f0f1a;
    --bg-secondary: #1a1a2e;
    --bg-tertiary: #252542;
    --text-primary: #e8e8f0;
    --text-secondary: #a0a0b8;
    --text-muted: #6b6b80;
    --accent: #6366f1;
    --accent-hover: #818cf8;
    --success: #22c55e;
    --error: #ef4444;
    --error-bg: #2d1f1f;
    --border: #353550;
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

  .form-container {
    padding: 0 24px;
    margin-bottom: 16px;
  }
</style>
