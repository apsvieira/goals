# i18n Follow-up: Accessibility Attributes, Tooltips, Alt Text, and Errors — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the internationalization of all user-facing strings that were missed in the initial i18n implementation: `aria-label` attributes (screen reader text), `title` attributes (tooltips), `alt` text (images), hardcoded fallback text, and error messages in `errors.ts`.

**Architecture:** Extend the existing `en.json` / `pt-BR.json` translation files with new keys under an `aria` namespace (for accessibility labels), `tooltip` namespace (for title attributes), `alt` namespace (for image alt text), and `error` namespace (for user-facing error messages). Svelte components already import `$_()` from `svelte-i18n`. For `errors.ts` (a plain `.ts` file where `$_()` is unavailable), use `get()` from `svelte/store` combined with the `format` store from `svelte-i18n` to read translations at call time.

**Tech Stack:** Svelte 5, TypeScript, svelte-i18n, Vitest (unit), Playwright (E2E)

---

### Task 1: Add new translation keys to en.json and pt-BR.json

**Files:**
- Modify: `frontend/src/lib/i18n/en.json`
- Modify: `frontend/src/lib/i18n/pt-BR.json`

**Step 1: Add new keys to `en.json`**

Add the following top-level sections to `en.json`:

```json
{
  "aria": {
    "closeForm": "Close form",
    "addGoal": "Add goal",
    "previousMonth": "Previous month",
    "nextMonth": "Next month",
    "goal": "Goal: {name}",
    "goalSelected": "Goal: {name} (selected for keyboard navigation)",
    "editGoal": "Edit {name}. Drag to reorder.",
    "progressBar": "{current} of {target} {period} goal completed",
    "progressWeekly": "weekly",
    "progressMonthly": "monthly",
    "day": "Day {day}",
    "userMenu": "User menu"
  },
  "tooltip": {
    "syncing": "Syncing...",
    "day": "Day {day}"
  },
  "alt": {
    "userAvatar": "User avatar",
    "userAvatarOf": "Avatar of {name}",
    "profileAvatar": "User avatar"
  },
  "error": {
    "network": "Unable to connect. Please check your internet connection.",
    "auth": "Your session has expired. Please sign in again.",
    "rateLimit": "Too many requests. Please wait a moment.",
    "notFound": "The item was not found. It may have been deleted.",
    "validationName": "Please enter a valid goal name.",
    "validationColor": "Please select a valid color.",
    "validationDate": "Please select a valid date.",
    "validationFuture": "Cannot mark future dates as complete.",
    "validationGeneric": "Please check your input and try again.",
    "conflict": "This item already exists.",
    "server": "Server error. Please try again later.",
    "sync": "Unable to sync your data. Changes will be saved locally.",
    "unknown": "Something went wrong. Please try again."
  },
  "fallback": {
    "user": "User"
  }
}
```

**Step 2: Add corresponding keys to `pt-BR.json`**

```json
{
  "aria": {
    "closeForm": "Fechar formulário",
    "addGoal": "Adicionar objetivo",
    "previousMonth": "Mês anterior",
    "nextMonth": "Próximo mês",
    "goal": "Objetivo: {name}",
    "goalSelected": "Objetivo: {name} (selecionado para navegação por teclado)",
    "editGoal": "Editar {name}. Arraste para reordenar.",
    "progressBar": "{current} de {target} do objetivo {period} concluído",
    "progressWeekly": "semanal",
    "progressMonthly": "mensal",
    "day": "Dia {day}",
    "userMenu": "Menu do usuário"
  },
  "tooltip": {
    "syncing": "Sincronizando...",
    "day": "Dia {day}"
  },
  "alt": {
    "userAvatar": "Avatar do usuário",
    "userAvatarOf": "Avatar de {name}",
    "profileAvatar": "Avatar do usuário"
  },
  "error": {
    "network": "Não foi possível conectar. Verifique sua conexão com a internet.",
    "auth": "Sua sessão expirou. Por favor, entre novamente.",
    "rateLimit": "Muitas solicitações. Por favor, aguarde um momento.",
    "notFound": "O item não foi encontrado. Pode ter sido excluído.",
    "validationName": "Por favor, insira um nome válido para o objetivo.",
    "validationColor": "Por favor, selecione uma cor válida.",
    "validationDate": "Por favor, selecione uma data válida.",
    "validationFuture": "Não é possível marcar datas futuras como concluídas.",
    "validationGeneric": "Por favor, verifique sua entrada e tente novamente.",
    "conflict": "Este item já existe.",
    "server": "Erro no servidor. Por favor, tente novamente mais tarde.",
    "sync": "Não foi possível sincronizar seus dados. As alterações serão salvas localmente.",
    "unknown": "Algo deu errado. Por favor, tente novamente."
  },
  "fallback": {
    "user": "Usuário"
  }
}
```

**Step 3: Run the existing key-parity test to confirm both files have matching keys**

Run: `cd frontend && npx vitest run src/lib/__tests__/i18n.test.ts`

Expected: The "pt-BR translation file has same keys as English" test passes.

**Step 4: Commit**

```bash
git add frontend/src/lib/i18n/en.json frontend/src/lib/i18n/pt-BR.json
git commit -m "feat(i18n): add translation keys for aria-labels, tooltips, alt text, errors, and fallbacks"
```

---

### Task 2: Translate aria-label and title attributes in Header.svelte

**Files:**
- Modify: `frontend/src/lib/components/Header.svelte`

**Step 1: Replace hardcoded aria-label on the add button**

Change:
```svelte
aria-label={showAddForm ? 'Close form' : 'Add goal'}
```
To:
```svelte
aria-label={showAddForm ? $_('aria.closeForm') : $_('aria.addGoal')}
```

**Step 2: Replace hardcoded title on the sync indicator**

Change:
```svelte
title="Syncing..."
```
To:
```svelte
title={$_('tooltip.syncing')}
```

**Step 3: Replace hardcoded alt on the avatar image**

Change:
```svelte
alt="User avatar"
```
To:
```svelte
alt={$_('alt.userAvatar')}
```

**Step 4: Commit**

```bash
git add frontend/src/lib/components/Header.svelte
git commit -m "feat(i18n): translate aria-labels, title, and alt text in Header"
```

---

### Task 3: Translate aria-label attributes in MonthNav.svelte

**Files:**
- Modify: `frontend/src/lib/components/MonthNav.svelte`

**Step 1: Replace hardcoded aria-labels on navigation buttons**

Change:
```svelte
aria-label="Previous month"
```
To:
```svelte
aria-label={$_('aria.previousMonth')}
```

Change:
```svelte
aria-label="Next month"
```
To:
```svelte
aria-label={$_('aria.nextMonth')}
```

**Step 2: Commit**

```bash
git add frontend/src/lib/components/MonthNav.svelte
git commit -m "feat(i18n): translate aria-labels in MonthNav"
```

---

### Task 4: Translate aria-label attributes in GoalRow.svelte

**Files:**
- Modify: `frontend/src/lib/components/GoalRow.svelte`

**Step 1: Add `$_` import**

```typescript
import { _ } from 'svelte-i18n';
```

**Step 2: Replace hardcoded aria-labels**

Change:
```svelte
aria-label="Goal: {goal.name}{isFocused ? ' (selected for keyboard navigation)' : ''}"
```
To:
```svelte
aria-label={isFocused ? $_('aria.goalSelected', { values: { name: goal.name }}) : $_('aria.goal', { values: { name: goal.name }})}
```

Change:
```svelte
aria-label="Edit {goal.name}. Drag to reorder."
```
To:
```svelte
aria-label={$_('aria.editGoal', { values: { name: goal.name }})}
```

**Step 3: Commit**

```bash
git add frontend/src/lib/components/GoalRow.svelte
git commit -m "feat(i18n): translate aria-labels in GoalRow"
```

---

### Task 5: Translate aria-label in ProgressBar.svelte

**Files:**
- Modify: `frontend/src/lib/components/ProgressBar.svelte`

**Step 1: Replace hardcoded aria-label**

Change:
```svelte
aria-label="{current} of {target} {period === 'week' ? 'weekly' : 'monthly'} goal completed"
```
To:
```svelte
aria-label={$_('aria.progressBar', { values: { current, target, period: period === 'week' ? $_('aria.progressWeekly') : $_('aria.progressMonthly') }})}
```

**Step 2: Commit**

```bash
git add frontend/src/lib/components/ProgressBar.svelte
git commit -m "feat(i18n): translate aria-label in ProgressBar"
```

---

### Task 6: Translate aria-label and title in DaySquare.svelte

**Files:**
- Modify: `frontend/src/lib/components/DaySquare.svelte`

**Step 1: Add `$_` import**

```typescript
import { _ } from 'svelte-i18n';
```

**Step 2: Replace hardcoded aria-label and title**

Change:
```svelte
aria-label="Day {day}"
title="Day {day}"
```
To:
```svelte
aria-label={$_('aria.day', { values: { day }})}
title={$_('tooltip.day', { values: { day }})}
```

**Step 3: Commit**

```bash
git add frontend/src/lib/components/DaySquare.svelte
git commit -m "feat(i18n): translate aria-label and title in DaySquare"
```

---

### Task 7: Translate aria-label and alt text in UserDropdown.svelte

**Files:**
- Modify: `frontend/src/lib/components/UserDropdown.svelte`

**Step 1: Replace hardcoded aria-label on dropdown container**

Change:
```svelte
aria-label="User menu"
```
To:
```svelte
aria-label={$_('aria.userMenu')}
```

**Step 2: Replace hardcoded alt text on avatar image**

The current code uses an English possessive pattern: `alt="{displayName}'s avatar"`. Replace with a translation that uses interpolation and avoids the English possessive.

Change:
```svelte
alt="{displayName}'s avatar"
```
To:
```svelte
alt={$_('alt.userAvatarOf', { values: { name: displayName }})}
```

**Step 3: Commit**

```bash
git add frontend/src/lib/components/UserDropdown.svelte
git commit -m "feat(i18n): translate aria-label and alt text in UserDropdown"
```

---

### Task 8: Translate alt text and fallback text in ProfilePage.svelte

**Files:**
- Modify: `frontend/src/lib/components/ProfilePage.svelte`

**Step 1: Replace hardcoded alt text on avatar image**

Change:
```svelte
alt="User avatar"
```
To:
```svelte
alt={$_('alt.profileAvatar')}
```

**Step 2: Replace hardcoded fallback 'User' text**

Change:
```svelte
{user?.name || user?.email?.split('@')[0] || 'User'}
```
To:
```svelte
{user?.name || user?.email?.split('@')[0] || $_('fallback.user')}
```

**Step 3: Commit**

```bash
git add frontend/src/lib/components/ProfilePage.svelte
git commit -m "feat(i18n): translate alt text and fallback text in ProfilePage"
```

---

### Task 9: Translate user-facing error messages in errors.ts

**Files:**
- Modify: `frontend/src/lib/errors.ts`

**Design note:** `errors.ts` is a plain TypeScript file, not a `.svelte` component, so the `$_()` auto-subscription syntax is unavailable. The correct approach is to import the `format` store from `svelte-i18n` and use `get()` from `svelte/store` to read it synchronously at call time. This works because `getUserFriendlyMessage` is always called reactively from App.svelte (inside catch blocks that set a reactive `error` variable), so the translation will reflect the current locale at the time of the error.

**Step 1: Read the current errors.ts to understand the exact error matching logic**

**Step 2: Update imports and replace hardcoded strings**

Replace the error message strings with translation calls using `get(format)`:

```typescript
import { get } from 'svelte/store';
import { format } from 'svelte-i18n';

export function getUserFriendlyMessage(error: unknown): string {
  const t = get(format);
  const msg = error instanceof Error ? error.message : String(error);
  const lowerMsg = msg.toLowerCase();

  // Network errors
  if (msg.includes('Failed to fetch') || msg.includes('NetworkError') || lowerMsg.includes('network')) {
    console.error('[Network Error]', msg);
    return t('error.network');
  }

  // Auth errors
  if (msg.includes('401') || lowerMsg.includes('unauthorized')) {
    console.error('[Auth Error]', msg);
    return t('error.auth');
  }

  // Rate limiting
  if (msg.includes('429') || lowerMsg.includes('rate limit')) {
    console.error('[Rate Limit]', msg);
    return t('error.rateLimit');
  }

  // Not found errors
  if (msg.includes('404') || lowerMsg.includes('not found')) {
    console.error('[Not Found]', msg);
    return t('error.notFound');
  }

  // Validation errors
  if (msg.includes('400') || lowerMsg.includes('invalid') || lowerMsg.includes('required')) {
    console.error('[Validation Error]', msg);
    if (lowerMsg.includes('name')) return t('error.validationName');
    if (lowerMsg.includes('color')) return t('error.validationColor');
    if (lowerMsg.includes('date')) return t('error.validationDate');
    if (lowerMsg.includes('future')) return t('error.validationFuture');
    return t('error.validationGeneric');
  }

  // Conflict errors
  if (msg.includes('409') || lowerMsg.includes('conflict') || lowerMsg.includes('already exists')) {
    console.error('[Conflict Error]', msg);
    return t('error.conflict');
  }

  // Server errors
  if (msg.includes('500') || msg.includes('502') || msg.includes('503') || lowerMsg.includes('internal server')) {
    console.error('[Server Error]', msg);
    return t('error.server');
  }

  // Sync errors
  if (lowerMsg.includes('sync')) {
    console.error('[Sync Error]', msg);
    return t('error.sync');
  }

  // Unknown
  console.error('[App Error]', msg);
  return t('error.unknown');
}
```

**Important:** Read the current `errors.ts` first to preserve any existing error matching logic that may differ from the above. The key change is replacing hardcoded English return strings with `t('error.xxx')` calls.

**Step 3: Commit**

```bash
git add frontend/src/lib/errors.ts
git commit -m "feat(i18n): translate user-facing error messages using get(format) from svelte-i18n"
```

---

### Task 10: Write unit tests for translated errors

**Files:**
- Create: `frontend/src/lib/__tests__/errors-i18n.test.ts`

**Step 1: Write the test**

```typescript
import { describe, it, expect, beforeAll, beforeEach, vi } from 'vitest';
import { setupI18n, saveLocale } from '../i18n';
import { getUserFriendlyMessage } from '../errors';

describe('errors.ts i18n', () => {
  beforeAll(() => {
    setupI18n();
  });

  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('returns English error messages when locale is en', async () => {
    saveLocale('en');
    await new Promise(resolve => setTimeout(resolve, 50));

    const msg = getUserFriendlyMessage(new Error('Failed to fetch'));
    expect(msg).toBe('Unable to connect. Please check your internet connection.');
  });

  it('returns Portuguese error messages when locale is pt-BR', async () => {
    saveLocale('pt-BR');
    await new Promise(resolve => setTimeout(resolve, 50));

    const msg = getUserFriendlyMessage(new Error('Failed to fetch'));
    expect(msg).toBe('Não foi possível conectar. Verifique sua conexão com a internet.');
  });

  it('translates auth errors', async () => {
    saveLocale('pt-BR');
    await new Promise(resolve => setTimeout(resolve, 50));

    const msg = getUserFriendlyMessage(new Error('401 Unauthorized'));
    expect(msg).toBe('Sua sessão expirou. Por favor, entre novamente.');
  });

  it('translates unknown errors', async () => {
    saveLocale('pt-BR');
    await new Promise(resolve => setTimeout(resolve, 50));

    const msg = getUserFriendlyMessage(new Error('some unexpected error'));
    expect(msg).toBe('Algo deu errado. Por favor, tente novamente.');
  });
});
```

**Step 2: Run the test**

Run: `cd frontend && npx vitest run src/lib/__tests__/errors-i18n.test.ts`

Expected: All 4 tests pass.

**Step 3: Commit**

```bash
git add frontend/src/lib/__tests__/errors-i18n.test.ts
git commit -m "test(i18n): add unit tests for translated error messages"
```

---

### Task 11: Add E2E test for translated aria-labels in Portuguese

**Files:**
- Modify: `frontend/e2e/language.spec.ts`

**Step 1: Add test for Portuguese aria-labels**

```typescript
test('aria-labels are translated in Portuguese', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('goal-tracker-locale', 'pt-BR');
  });
  await page.goto('/');
  await page.waitForSelector('header', { timeout: 10000 });

  // Month nav buttons should have Portuguese aria-labels
  await expect(page.locator('button[aria-label="Mês anterior"]')).toBeVisible();
  await expect(page.locator('button[aria-label="Próximo mês"]')).toBeAttached();
});
```

**Step 2: Commit**

```bash
git add frontend/e2e/language.spec.ts
git commit -m "test(i18n): add E2E test for translated aria-labels in Portuguese"
```

---

### Task 12: Run full test suite

**Step 1: Run unit tests**

Run: `cd frontend && npx vitest run`

Expected: All tests pass, including the existing key-parity test.

**Step 2: Run E2E tests (if backend available)**

Run: `cd frontend && npx playwright test`

Expected: All tests pass.

**Step 3: Fix any failures, then commit**
