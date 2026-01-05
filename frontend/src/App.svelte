<script lang="ts">
  import { onMount } from 'svelte';
  import MonthNav from './lib/components/MonthNav.svelte';
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

<main>
  <header>
    <MonthNav
      month={currentMonth}
      onPrev={prevMonth}
      onNext={nextMonth}
    />
    <button class="add-btn" on:click={() => showAddForm = !showAddForm} aria-label={showAddForm ? 'Close form' : 'Add goal'}>
      {showAddForm ? '×' : '+'}
    </button>
  </header>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if showAddForm}
    <AddGoalForm
      onAdd={handleAddGoal}
      onCancel={() => showAddForm = false}
    />
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
    padding: 20px;
    background: var(--bg-primary);
    color: var(--text-primary);
  }

  main {
    max-width: 800px;
    margin: 0 auto;
  }

  header {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 16px;
    margin-bottom: 24px;
  }

  .add-btn {
    width: 32px;
    height: 32px;
    padding: 0;
    font-size: 20px;
    line-height: 1;
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 50%;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .add-btn:hover {
    background: var(--accent-hover);
  }

  .error {
    padding: 12px;
    margin: 16px 0;
    background: var(--error-bg);
    color: var(--error);
    border-radius: 4px;
    border: 1px solid var(--error);
  }

  .loading, .empty {
    color: var(--text-secondary);
    font-style: italic;
  }

  .goals {
    margin-top: 24px;
  }
</style>
