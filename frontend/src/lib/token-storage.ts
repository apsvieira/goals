import { Preferences } from '@capacitor/preferences';

const TOKEN_KEY = 'auth_token';

/**
 * Save authentication token to secure storage
 */
export async function saveToken(token: string): Promise<void> {
  await Preferences.set({
    key: TOKEN_KEY,
    value: token,
  });
}

/**
 * Get authentication token from secure storage
 */
export async function getToken(): Promise<string | null> {
  const { value } = await Preferences.get({ key: TOKEN_KEY });
  return value;
}

/**
 * Clear authentication token from secure storage
 */
export async function clearToken(): Promise<void> {
  await Preferences.remove({ key: TOKEN_KEY });
}
