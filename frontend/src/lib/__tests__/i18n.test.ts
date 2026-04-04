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
