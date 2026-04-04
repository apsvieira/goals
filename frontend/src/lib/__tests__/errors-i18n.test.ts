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
