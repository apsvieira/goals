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
