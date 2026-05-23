# Notifications UX Polish Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Polish the notification prompt and settings UI: animate the prompt entrance, replace the horizontal segmented frequency selector with a vertical option list using flat styling.

**Architecture:** Pure CSS/template changes across two Svelte components. No logic, store, i18n, or test changes needed — these are visual-only improvements.

**Tech Stack:** Svelte 3 (scoped CSS, templates), CSS keyframe animations

---

### Task 1: Animate Notification Prompt Entrance

**Files:**
- Modify: `frontend/src/lib/components/NotificationPrompt.svelte` (CSS only, lines 37–107)

The prompt currently appears instantly. Add CSS keyframe animations so the backdrop fades in and the dialog slides up over ~500ms.

**Step 1: Add entrance animations**

In the `<style>` block of `NotificationPrompt.svelte`, add two keyframes at the top of the style block:

```css
@keyframes fadeIn {
  from { opacity: 0; }
  to   { opacity: 1; }
}

@keyframes slideUp {
  from { opacity: 0; transform: translateY(1rem); }
  to   { opacity: 1; transform: translateY(0); }
}
```

Add to `.backdrop`:
```css
animation: fadeIn 0.4s ease-out;
```

Add to `.dialog`:
```css
animation: slideUp 0.5s ease-out;
```

The backdrop fades in over 400ms while the dialog slides up over 500ms, creating a layered entrance that isn't jarring.

**Step 2: Verify visually**

Run: `cd frontend && npx vite dev`
Trigger the notification prompt flow and confirm it eases in smoothly.

**Step 3: Commit**

```
feat(notifications): animate prompt entrance with fade + slide
```

---

### Task 2: Convert Frequency Selector to Vertical Option List

**Files:**
- Modify: `frontend/src/lib/components/NotificationSettings.svelte` (template lines 119–147, CSS lines 214–243)

Replace the horizontal `inline-flex` segmented control with a vertical stack of full-width options. The label moves above the options instead of being inline. The selected option has a clear accent background; unselected options are plain text rows separated by borders — no rounded pill buttons.

**Step 1: Replace the frequency row template**

Replace lines 119–147 (the `<div class="row">` containing the segmented control) with:

```svelte
<div class="frequency-group">
  <span class="group-label">{$_('notifications.frequencyLabel')}</span>
  <div class="option-list" role="radiogroup" aria-label={$_('notifications.frequencyLabel')}>
    <button
      type="button"
      class="option"
      class:selected={settings.frequency === 'off'}
      role="radio"
      aria-checked={settings.frequency === 'off'}
      on:click={() => handleFrequencyClick('off')}
    >
      {$_('notifications.frequency.off')}
    </button>
    <button
      type="button"
      class="option"
      class:selected={settings.frequency === 'daily'}
      role="radio"
      aria-checked={settings.frequency === 'daily'}
      on:click={() => handleFrequencyClick('daily')}
    >
      {$_('notifications.frequency.daily')}
    </button>
    <button
      type="button"
      class="option"
      class:selected={settings.frequency === 'weekly'}
      role="radio"
      aria-checked={settings.frequency === 'weekly'}
      on:click={() => handleFrequencyClick('weekly')}
    >
      {$_('notifications.frequency.weekly')}
    </button>
  </div>
</div>
```

Key changes vs. current:
- Wrapping div changes from `.row` to `.frequency-group` (label above, not inline)
- `role="radiogroup"` / `role="radio"` / `aria-checked` for accessibility
- Each button is a full-width `.option` instead of a `.segment`

**Step 2: Replace the CSS**

Remove these CSS rules: `.segmented`, `.segment`, `.segment + .segment`, `.segment:hover`, `.segment.active`.

Add these replacements:

```css
.frequency-group {
  margin-bottom: 1rem;
}

.group-label {
  display: block;
  font-size: 0.875rem;
  color: var(--text-secondary);
  margin-bottom: 0.5rem;
}

.option-list {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  overflow: hidden;
  background: var(--bg-secondary);
}

.option {
  display: block;
  width: 100%;
  padding: 0.625rem 0.875rem;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 0.875rem;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.15s, color 0.15s;
}

.option + .option {
  border-top: 1px solid var(--border);
}

.option:hover {
  background: var(--bg-tertiary);
}

.option.selected {
  background: var(--accent);
  color: white;
}
```

This produces an iOS-settings-style grouped list: clean vertical stack, subtle separators, accent fill on the selected option.

**Step 3: Verify visually**

Navigate to the Notifications settings page. Confirm:
- Three options stacked vertically: Off, Daily, Weekly
- Full-width text — no clipping
- Selected option has green accent background + white text
- Unselected options are plain secondary-colored text
- Subtle hover effect on unselected options
- Border separators between options (no gap)

**Step 4: Commit**

```
feat(notifications): vertical frequency selector with flat option styling
```

---

## Summary

| File | What Changes |
|------|-------------|
| `NotificationPrompt.svelte` | +2 keyframes, +2 animation properties on backdrop/dialog |
| `NotificationSettings.svelte` | Template: segmented → vertical option list. CSS: replace 5 rules with 7 new rules |

No logic, i18n, store, or test changes. All visual/CSS.
