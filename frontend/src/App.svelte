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

  function getCompletedDays(goalId: string): Set<number> {
    const map = completionsByGoal[goalId];
    return map ? new Set(map.keys()) : new Set();
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
    {#if !showAddForm}
      <button class="add-btn" on:click={() => showAddForm = true}>
        + Add Goal
      </button>
    {/if}
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
          completedDays={getCompletedDays(goal.id)}
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
  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
    margin: 0;
    padding: 20px;
    background: #fafafa;
    color: #333;
  }

  main {
    max-width: 800px;
    margin: 0 auto;
  }

  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 16px;
  }

  .add-btn {
    padding: 8px 16px;
    font-size: 14px;
    background: #4CAF50;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .add-btn:hover {
    background: #45a049;
  }

  .error {
    padding: 12px;
    margin: 16px 0;
    background: #ffebee;
    color: #c62828;
    border-radius: 4px;
  }

  .loading, .empty {
    color: #666;
    font-style: italic;
  }

  .goals {
    margin-top: 24px;
  }
</style>
