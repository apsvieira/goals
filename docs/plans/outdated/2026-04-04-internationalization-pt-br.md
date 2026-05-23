# Internationalization (pt-BR) — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add internationalization support with English (default) and Brazilian Portuguese (pt-BR). Users choose their language from the user menu, and the preference persists across sessions.

**Architecture:** Use `svelte-i18n` — the standard i18n library for Svelte. Translation strings live in JSON files (`en.json`, `pt-BR.json`). Language preference is stored in `localStorage`. A language selector is added to the user dropdown menu. Date/number formatting uses the selected locale. The PrivacyPolicy page is excluded from translation for now (legal text requires professional translation).

**Tech Stack:** Svelte 5, TypeScript, svelte-i18n, Vitest, Playwright

---

### Task 1: Install svelte-i18n and create translation files

**Files:**
- Modify: `frontend/package.json` (dependency added by npm)
- Create: `frontend/src/lib/i18n/en.json`
- Create: `frontend/src/lib/i18n/pt-BR.json`
- Create: `frontend/src/lib/i18n/index.ts`

**Step 1: Install svelte-i18n**

Run: `cd frontend && npm install svelte-i18n`

**Step 2: Create English translation file**

Create `frontend/src/lib/i18n/en.json`:

```json
{
  "app": {
    "loading": "Loading your goals..."
  },
  "auth": {
    "title": "tiny tracker",
    "subtitle": "Track your daily habits and achieve your goals",
    "signInGoogle": "Sign in with Google",
    "devLogin": "Dev Login",
    "privacyPolicy": "Privacy Policy"
  },
  "header": {
    "newGoal": "New Goal",
    "cancel": "Cancel"
  },
  "month": {
    "jan": "Jan",
    "feb": "Feb",
    "mar": "Mar",
    "apr": "Apr",
    "may": "May",
    "jun": "Jun",
    "jul": "Jul",
    "aug": "Aug",
    "sep": "Sep",
    "oct": "Oct",
    "nov": "Nov",
    "dec": "Dec"
  },
  "menu": {
    "profile": "Profile",
    "language": "Language",
    "logOut": "Log Out"
  },
  "welcome": {
    "title": "Welcome to Goal Tracker!",
    "description": "Track daily habits and goals with a visual calendar.",
    "featureCreate": "Create goals",
    "featureCreateDesc": "Click \"{newGoal}\" to start tracking",
    "featureMark": "Mark completions",
    "featureMarkDesc": "Click day squares to toggle",
    "featureTargets": "Set targets",
    "featureTargetsDesc": "Optional weekly/monthly targets with progress bars",
    "featureSwipe": "Swipe to navigate",
    "featureSwipeDesc": "View past months",
    "cta": "Create Your First Goal"
  },
  "offline": {
    "message": "You're offline. Changes will be saved locally."
  },
  "goalEditor": {
    "nameLabel": "Name",
    "namePlaceholder": "Goal name",
    "targetFrequency": "Target frequency",
    "daily": "Daily (no target)",
    "weekly": "Weekly target",
    "monthly": "Monthly target",
    "timesPerWeek": "Times per week",
    "timesPerMonth": "Times per month",
    "previewDefault": "Goal Name",
    "delete": "Delete",
    "cancel": "Cancel",
    "addGoal": "Add Goal",
    "save": "Save",
    "deleteConfirm": "Are you sure you want to delete this goal?"
  },
  "progress": {
    "perWeek": "/wk",
    "perMonth": "/mo"
  },
  "profile": {
    "back": "Back",
    "memberSince": "Member since {date}",
    "overview": "Overview",
    "totalCompletions": "Total Completions",
    "avgRate": "Avg Completion Rate",
    "bestStreak": "Best Streak (days)",
    "activeGoals": "Active Goals",
    "goalStats": "Goal Statistics",
    "noGoals": "No goals yet. Create your first goal to start tracking!",
    "days": "{count} days ({rate}% rate)",
    "dayStreak": "{count} day streak",
    "bestDays": "Best: {count} days",
    "bestWeek": "Best week: {count}",
    "bestMonth": "Best month: {count}",
    "periodSuccess": "{rate}% of {period} target met ({successful}/{total})",
    "periodWeeks": "weeks",
    "periodMonths": "months",
    "dataExport": "Data Export",
    "exportDescription": "Download a copy of all your goals and completions.",
    "exportButton": "Export Data (JSON)"
  },
  "footer": {
    "privacy": "Privacy"
  },
  "language": {
    "en": "English",
    "pt-BR": "Português (Brasil)"
  }
}
```

**Step 3: Create Brazilian Portuguese translation file**

Create `frontend/src/lib/i18n/pt-BR.json`:

```json
{
  "app": {
    "loading": "Carregando seus objetivos..."
  },
  "auth": {
    "title": "tiny tracker",
    "subtitle": "Acompanhe seus hábitos diários e alcance seus objetivos",
    "signInGoogle": "Entrar com Google",
    "devLogin": "Login Dev",
    "privacyPolicy": "Política de Privacidade"
  },
  "header": {
    "newGoal": "Novo Objetivo",
    "cancel": "Cancelar"
  },
  "month": {
    "jan": "Jan",
    "feb": "Fev",
    "mar": "Mar",
    "apr": "Abr",
    "may": "Mai",
    "jun": "Jun",
    "jul": "Jul",
    "aug": "Ago",
    "sep": "Set",
    "oct": "Out",
    "nov": "Nov",
    "dec": "Dez"
  },
  "menu": {
    "profile": "Perfil",
    "language": "Idioma",
    "logOut": "Sair"
  },
  "welcome": {
    "title": "Bem-vindo ao Goal Tracker!",
    "description": "Acompanhe hábitos diários e objetivos com um calendário visual.",
    "featureCreate": "Crie objetivos",
    "featureCreateDesc": "Clique em \"{newGoal}\" para começar",
    "featureMark": "Marque conclusões",
    "featureMarkDesc": "Clique nos quadrados dos dias para alternar",
    "featureTargets": "Defina metas",
    "featureTargetsDesc": "Metas semanais/mensais opcionais com barras de progresso",
    "featureSwipe": "Deslize para navegar",
    "featureSwipeDesc": "Veja meses anteriores",
    "cta": "Crie Seu Primeiro Objetivo"
  },
  "offline": {
    "message": "Você está offline. As alterações serão salvas localmente."
  },
  "goalEditor": {
    "nameLabel": "Nome",
    "namePlaceholder": "Nome do objetivo",
    "targetFrequency": "Frequência alvo",
    "daily": "Diário (sem meta)",
    "weekly": "Meta semanal",
    "monthly": "Meta mensal",
    "timesPerWeek": "Vezes por semana",
    "timesPerMonth": "Vezes por mês",
    "previewDefault": "Nome do Objetivo",
    "delete": "Excluir",
    "cancel": "Cancelar",
    "addGoal": "Adicionar Objetivo",
    "save": "Salvar",
    "deleteConfirm": "Tem certeza de que deseja excluir este objetivo?"
  },
  "progress": {
    "perWeek": "/sem",
    "perMonth": "/mês"
  },
  "profile": {
    "back": "Voltar",
    "memberSince": "Membro desde {date}",
    "overview": "Visão Geral",
    "totalCompletions": "Total de Conclusões",
    "avgRate": "Taxa Média de Conclusão",
    "bestStreak": "Melhor Sequência (dias)",
    "activeGoals": "Objetivos Ativos",
    "goalStats": "Estatísticas dos Objetivos",
    "noGoals": "Nenhum objetivo ainda. Crie seu primeiro objetivo para começar!",
    "days": "{count} dias ({rate}% taxa)",
    "dayStreak": "sequência de {count} dias",
    "bestDays": "Melhor: {count} dias",
    "bestWeek": "Melhor semana: {count}",
    "bestMonth": "Melhor mês: {count}",
    "periodSuccess": "{rate}% das {period} com meta atingida ({successful}/{total})",
    "periodWeeks": "semanas",
    "periodMonths": "meses",
    "dataExport": "Exportar Dados",
    "exportDescription": "Baixe uma cópia de todos os seus objetivos e conclusões.",
    "exportButton": "Exportar Dados (JSON)"
  },
  "footer": {
    "privacy": "Privacidade"
  },
  "language": {
    "en": "English",
    "pt-BR": "Português (Brasil)"
  }
}
```

**Step 4: Create the i18n initialization module**

Create `frontend/src/lib/i18n/index.ts`:

```typescript
import { register, init, getLocaleFromNavigator, locale } from 'svelte-i18n';

import en from './en.json';
import ptBR from './pt-BR.json';

const STORAGE_KEY = 'goal-tracker-locale';

export const supportedLocales = [
  { code: 'en', label: 'English' },
  { code: 'pt-BR', label: 'Português (Brasil)' },
] as const;

register('en', () => Promise.resolve(en));
register('pt-BR', () => Promise.resolve(ptBR));

function getInitialLocale(): string {
  // 1. Check localStorage for saved preference
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved && supportedLocales.some(l => l.code === saved)) {
    return saved;
  }

  // 2. Check browser locale
  const browserLocale = getLocaleFromNavigator();
  if (browserLocale) {
    // Match pt-BR, pt, etc.
    if (browserLocale.startsWith('pt')) return 'pt-BR';
    if (browserLocale.startsWith('en')) return 'en';
  }

  // 3. Default to English
  return 'en';
}

export function saveLocale(loc: string) {
  localStorage.setItem(STORAGE_KEY, loc);
  locale.set(loc);
}

export function setupI18n() {
  init({
    fallbackLocale: 'en',
    initialLocale: getInitialLocale(),
  });
}
```

**Step 5: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/lib/i18n/
git commit -m "feat(i18n): add svelte-i18n with English and pt-BR translations"
```

---

### Task 2: Initialize i18n in the app entry point

**Files:**
- Modify: `frontend/src/main.ts`

**Step 1: Read `main.ts` to see current contents**

**Step 2: Add i18n setup before app initialization**

Add at the top of `main.ts` (before the App mount):

```typescript
import { setupI18n } from './lib/i18n';
setupI18n();
```

**Step 3: Commit**

```bash
git add frontend/src/main.ts
git commit -m "feat(i18n): initialize i18n in app entry point"
```

---

### Task 3: Translate AuthPage.svelte

**Files:**
- Modify: `frontend/src/lib/components/AuthPage.svelte`

**Step 1: Replace hardcoded strings with translation calls**

Add import at the top of the script:

```typescript
import { _ } from 'svelte-i18n';
```

Replace template strings:

| Current | Replacement |
|---------|-------------|
| `tiny tracker` | `{$_('auth.title')}` |
| `Track your daily habits and achieve your goals` | `{$_('auth.subtitle')}` |
| `Sign in with Google` | `{$_('auth.signInGoogle')}` |
| `Dev Login` | `{$_('auth.devLogin')}` |
| `Privacy Policy` | `{$_('auth.privacyPolicy')}` |

**Step 2: Verify the page renders correctly**

Run: `cd frontend && npm run dev` — check the auth page loads with English text.

**Step 3: Commit**

```bash
git add frontend/src/lib/components/AuthPage.svelte
git commit -m "feat(i18n): translate AuthPage"
```

---

### Task 4: Translate Header.svelte and MonthNav.svelte

**Files:**
- Modify: `frontend/src/lib/components/Header.svelte`
- Modify: `frontend/src/lib/components/MonthNav.svelte`

**Step 1: Translate Header.svelte**

Add import:

```typescript
import { _ } from 'svelte-i18n';
```

Replace strings:

| Current | Replacement |
|---------|-------------|
| `{showAddForm ? 'Cancel' : 'New Goal'}` | `{showAddForm ? $_('header.cancel') : $_('header.newGoal')}` |

**Step 2: Translate MonthNav.svelte**

Add import:

```typescript
import { _ } from 'svelte-i18n';
```

Replace the hardcoded `monthNames` array and its reactive block (lines 6-14):

```typescript
const monthKeys = [
  'month.jan', 'month.feb', 'month.mar', 'month.apr',
  'month.may', 'month.jun', 'month.jul', 'month.aug',
  'month.sep', 'month.oct', 'month.nov', 'month.dec'
];

$: {
  const [year, monthNum] = month.split('-').map(Number);
  displayMonth = $_(monthKeys[monthNum - 1]);
}
```

**Step 3: Commit**

```bash
git add frontend/src/lib/components/Header.svelte frontend/src/lib/components/MonthNav.svelte
git commit -m "feat(i18n): translate Header and MonthNav"
```

---

### Task 5: Translate GoalEditor.svelte

**Files:**
- Modify: `frontend/src/lib/components/GoalEditor.svelte`

**Step 1: Add import and replace all strings**

Add import:

```typescript
import { _ } from 'svelte-i18n';
```

Key replacements:

| Current | Replacement |
|---------|-------------|
| `Name` (label) | `{$_('goalEditor.nameLabel')}` |
| `placeholder="Goal name"` | `placeholder={$_('goalEditor.namePlaceholder')}` |
| `Target frequency` (legend) | `{$_('goalEditor.targetFrequency')}` |
| `Daily (no target)` | `{$_('goalEditor.daily')}` |
| `Weekly target` | `{$_('goalEditor.weekly')}` |
| `Monthly target` | `{$_('goalEditor.monthly')}` |
| `Times per week` / `Times per month` | `{$_(targetType === 'weekly' ? 'goalEditor.timesPerWeek' : 'goalEditor.timesPerMonth')}` |
| `Goal Name` (preview) | `{$_('goalEditor.previewDefault')}` |
| `Delete` button | `{$_('goalEditor.delete')}` |
| `Cancel` buttons (both) | `{$_('goalEditor.cancel')}` |
| `{mode === 'add' ? 'Add Goal' : 'Save'}` | `{mode === 'add' ? $_('goalEditor.addGoal') : $_('goalEditor.save')}` |
| `Are you sure you want to delete this goal?` | `{$_('goalEditor.deleteConfirm')}` |
| `Delete` confirm button | `{$_('goalEditor.delete')}` |

**Step 2: Commit**

```bash
git add frontend/src/lib/components/GoalEditor.svelte
git commit -m "feat(i18n): translate GoalEditor"
```

---

### Task 6: Translate App.svelte (main view strings)

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: Add import**

```typescript
import { _ } from 'svelte-i18n';
```

**Step 2: Replace hardcoded strings**

| Current | Replacement |
|---------|-------------|
| `Loading your goals...` (line 678) | `{$_('app.loading')}` |
| `Welcome to Goal Tracker!` (line 732) | `{$_('welcome.title')}` |
| `Track daily habits...` (line 733) | `{$_('welcome.description')}` |
| Welcome features list (lines 735-739) | Use translation keys with interpolation — see below |
| `Create Your First Goal` (line 741) | `{$_('welcome.cta')}` |
| `You're offline...` (line 722) | `{$_('offline.message')}` |

For the features list, replace lines 734-739:

```svelte
<ul class="welcome-features">
  <li><strong>{$_('welcome.featureCreate')}</strong> - {$_('welcome.featureCreateDesc', { values: { newGoal: $_('header.newGoal') }})}</li>
  <li><strong>{$_('welcome.featureMark')}</strong> - {$_('welcome.featureMarkDesc')}</li>
  <li><strong>{$_('welcome.featureTargets')}</strong> - {$_('welcome.featureTargetsDesc')}</li>
  <li><strong>{$_('welcome.featureSwipe')}</strong> - {$_('welcome.featureSwipeDesc')}</li>
</ul>
```

**Step 3: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat(i18n): translate App.svelte main view strings"
```

---

### Task 7: Translate UserDropdown.svelte

**Files:**
- Modify: `frontend/src/lib/components/UserDropdown.svelte`

**Step 1: Add import and replace strings**

```typescript
import { _ } from 'svelte-i18n';
```

| Current | Replacement |
|---------|-------------|
| `Profile` | `{$_('menu.profile')}` |
| `Log Out` | `{$_('menu.logOut')}` |

**Step 2: Commit**

```bash
git add frontend/src/lib/components/UserDropdown.svelte
git commit -m "feat(i18n): translate UserDropdown"
```

---

### Task 8: Translate ProfilePage.svelte

**Files:**
- Modify: `frontend/src/lib/components/ProfilePage.svelte`

**Step 1: Add import**

```typescript
import { _, locale } from 'svelte-i18n';
```

**Step 2: Update `formatMemberSince` to use current locale**

```typescript
function formatMemberSince(dateStr: string | undefined): string {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  const currentLocale = $locale || 'en';
  return date.toLocaleDateString(currentLocale, { day: 'numeric', month: 'short', year: 'numeric' });
}
```

Note: `$locale` is the svelte-i18n reactive store — using it directly in a function called from the template will work because the function is re-invoked when the template re-renders. However, `$locale` syntax only works in `.svelte` files (it's Svelte's auto-subscription). Since ProfilePage is a `.svelte` file, this works.

**Step 3: Replace all hardcoded strings**

Key replacements:

| Current | Replacement |
|---------|-------------|
| `Back` | `{$_('profile.back')}` |
| `Member since {formatMemberSince(user.created_at)}` | `{$_('profile.memberSince', { values: { date: formatMemberSince(user.created_at) }})}` |
| `Overview` | `{$_('profile.overview')}` |
| `Total Completions` | `{$_('profile.totalCompletions')}` |
| `Avg Completion Rate` | `{$_('profile.avgRate')}` |
| `Best Streak (days)` | `{$_('profile.bestStreak')}` |
| `Active Goals` | `{$_('profile.activeGoals')}` |
| `Goal Statistics` | `{$_('profile.goalStats')}` |
| `No goals yet...` | `{$_('profile.noGoals')}` |
| `{stats.daysCompleted} days ({stats.rate}% rate)` | `{$_('profile.days', { values: { count: stats.daysCompleted, rate: stats.rate }})}` |
| `{stats.currentStreak} day streak` | `{$_('profile.dayStreak', { values: { count: stats.currentStreak }})}` |
| `Best: {stats.longestStreak} days` | `{$_('profile.bestDays', { values: { count: stats.longestStreak }})}` |
| `Best week: {stats.bestWeek}` | `{$_('profile.bestWeek', { values: { count: stats.bestWeek }})}` |
| `Best month: {stats.bestMonth}` | `{$_('profile.bestMonth', { values: { count: stats.bestMonth }})}` |
| Period success text | `{$_('profile.periodSuccess', { values: { rate: stats.periodStats.successRate, period: $_('profile.period' + (stats.periodStats.period === 'weeks' ? 'Weeks' : 'Months')), successful: stats.periodStats.successful, total: stats.periodStats.total }})}` |
| `Data Export` | `{$_('profile.dataExport')}` |
| `Download a copy...` | `{$_('profile.exportDescription')}` |
| `Export Data (JSON)` | `{$_('profile.exportButton')}` |

**Step 4: Commit**

```bash
git add frontend/src/lib/components/ProfilePage.svelte
git commit -m "feat(i18n): translate ProfilePage with locale-aware date formatting"
```

---

### Task 9: Translate ProgressBar.svelte and Footer.svelte

**Files:**
- Modify: `frontend/src/lib/components/ProgressBar.svelte`
- Modify: `frontend/src/lib/components/Footer.svelte`

**Step 1: Translate ProgressBar.svelte**

Add import and replace:

```typescript
import { _ } from 'svelte-i18n';

$: periodShort = period === 'week' ? $_('progress.perWeek') : $_('progress.perMonth');
```

**Step 2: Translate Footer.svelte**

Add import and replace:

```typescript
import { _ } from 'svelte-i18n';
```

Replace `Privacy` text with `{$_('footer.privacy')}`.

**Step 3: Commit**

```bash
git add frontend/src/lib/components/ProgressBar.svelte frontend/src/lib/components/Footer.svelte
git commit -m "feat(i18n): translate ProgressBar and Footer"
```

---

### Task 10: Add language selector to UserDropdown

**Files:**
- Modify: `frontend/src/lib/components/UserDropdown.svelte`

**Step 1: Add language selector between Profile and Log Out**

Add imports:

```typescript
import { _, locale } from 'svelte-i18n';
import { supportedLocales, saveLocale } from '../i18n';
```

Add between the Profile button and the Log Out divider:

```svelte
<div class="divider"></div>
<div class="menu-section">
  <span class="menu-label">{$_('menu.language')}</span>
  <div class="language-options">
    {#each supportedLocales as loc}
      <button
        class="language-btn"
        class:active={$locale === loc.code}
        on:click={() => saveLocale(loc.code)}
        role="menuitemradio"
        aria-checked={$locale === loc.code}
      >
        {loc.label}
      </button>
    {/each}
  </div>
</div>
```

Add styles:

```css
.menu-section {
  padding: 0.5rem 1rem;
}

.menu-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.language-options {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  margin-top: 0.375rem;
}

.language-btn {
  display: flex;
  align-items: center;
  width: 100%;
  padding: 0.375rem 0.5rem;
  background: transparent;
  border: none;
  border-radius: 0.25rem;
  color: var(--text-primary);
  font-size: 0.8125rem;
  cursor: pointer;
  text-align: left;
}

.language-btn:hover {
  background: var(--bg-secondary);
}

.language-btn.active {
  background: var(--bg-tertiary);
  font-weight: 600;
}
```

**Step 2: Commit**

```bash
git add frontend/src/lib/components/UserDropdown.svelte
git commit -m "feat(i18n): add language selector to user dropdown menu"
```

---

### Task 11: Add unit tests for i18n initialization

**Files:**
- Create: `frontend/src/lib/__tests__/i18n.test.ts`

**Step 1: Write the test**

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest';

describe('i18n', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.resetModules();
  });

  it('exports supportedLocales with en and pt-BR', async () => {
    const { supportedLocales } = await import('../i18n');
    expect(supportedLocales).toHaveLength(2);
    expect(supportedLocales.map(l => l.code)).toEqual(['en', 'pt-BR']);
  });

  it('saveLocale persists to localStorage', async () => {
    const { saveLocale } = await import('../i18n');
    saveLocale('pt-BR');
    expect(localStorage.getItem('goal-tracker-locale')).toBe('pt-BR');
  });

  it('English translation file has all expected top-level keys', async () => {
    const en = (await import('../i18n/en.json')).default;
    expect(Object.keys(en)).toEqual(
      expect.arrayContaining(['app', 'auth', 'header', 'month', 'menu', 'welcome', 'offline', 'goalEditor', 'progress', 'profile', 'footer', 'language'])
    );
  });

  it('pt-BR translation file has same keys as English', async () => {
    const en = (await import('../i18n/en.json')).default;
    const ptBR = (await import('../i18n/pt-BR.json')).default;

    function getKeys(obj: Record<string, unknown>, prefix = ''): string[] {
      return Object.entries(obj).flatMap(([key, value]) => {
        const fullKey = prefix ? `${prefix}.${key}` : key;
        if (typeof value === 'object' && value !== null) {
          return getKeys(value as Record<string, unknown>, fullKey);
        }
        return [fullKey];
      });
    }

    const enKeys = getKeys(en).sort();
    const ptBRKeys = getKeys(ptBR).sort();
    expect(ptBRKeys).toEqual(enKeys);
  });
});
```

**Step 2: Run unit tests**

Run: `cd frontend && npx vitest run src/lib/__tests__/i18n.test.ts`

Expected: All 4 tests pass.

**Step 3: Commit**

```bash
git add frontend/src/lib/__tests__/i18n.test.ts
git commit -m "test(i18n): add unit tests for i18n setup and translation key parity"
```

---

### Task 12: Add E2E test for language switching

**Files:**
- Create: `frontend/e2e/language.spec.ts`

**Step 1: Write the E2E test**

```typescript
import { test, expect } from './fixtures/base';

test.describe('Language Switching', () => {
  test('defaults to English', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Header should show English text
    await expect(page.locator('button:has-text("New Goal")')).toBeVisible();
  });

  test('can switch to Portuguese via user menu', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Open user menu
    await page.locator('.user-indicator').click();

    // Click pt-BR language option
    await page.locator('button.language-btn:has-text("Português")').click();

    // Header should now show Portuguese text
    await expect(page.locator('button:has-text("Novo Objetivo")')).toBeVisible();
  });

  test('language preference persists across page reload', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('header', { timeout: 10000 });

    // Switch to Portuguese
    await page.locator('.user-indicator').click();
    await page.locator('button.language-btn:has-text("Português")').click();
    await expect(page.locator('button:has-text("Novo Objetivo")')).toBeVisible();

    // Reload page
    await page.reload();
    await page.waitForSelector('header', { timeout: 10000 });

    // Should still be in Portuguese
    await expect(page.locator('button:has-text("Novo Objetivo")')).toBeVisible();
  });
});
```

**Step 2: Run E2E tests**

Run: `cd frontend && npx playwright test e2e/language.spec.ts --project=chromium`

Expected: All 3 tests pass.

**Step 3: Commit**

```bash
git add frontend/e2e/language.spec.ts
git commit -m "test(i18n): add E2E tests for language switching and persistence"
```

---

### Task 13: Update E2E fixtures and page objects for i18n

**Files:**
- Modify: `frontend/e2e/fixtures/base.ts`
- Modify: `frontend/e2e/pages/HomePage.ts`

**Step 1: Update base fixture**

The base fixture references `"New Goal"` and `"Add Goal"` by text. Since E2E tests run in English by default (no `localStorage` set), these should continue to work. However, for robustness, ensure tests set English locale in `localStorage` before navigating:

In the `goalTrackerPage` fixture, before `page.goto('/')`:

```typescript
goalTrackerPage: async ({ page }, use) => {
  // Ensure English locale for consistent E2E tests
  await page.addInitScript(() => {
    localStorage.setItem('goal-tracker-locale', 'en');
  });
  await page.goto('/');
  await use(page);
},
```

Similarly in the `testGoal` fixture, add before `page.goto('/')`:

```typescript
await page.addInitScript(() => {
  localStorage.setItem('goal-tracker-locale', 'en');
});
```

**Step 2: Commit**

```bash
git add frontend/e2e/fixtures/base.ts frontend/e2e/pages/HomePage.ts
git commit -m "test(i18n): ensure E2E fixtures use English locale explicitly"
```

---

### Task 14: Run full test suite

**Step 1: Run unit tests**

Run: `cd frontend && npx vitest run`

Expected: All tests pass.

**Step 2: Run E2E tests**

Run: `cd frontend && npx playwright test`

Expected: All tests pass (including new language tests).

**Step 3: Fix any failures, then commit**

If any tests fail, fix the issue and commit the fix.
