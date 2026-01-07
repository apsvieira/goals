import { Browser } from '@capacitor/browser';
import { Capacitor } from '@capacitor/core';

const PRODUCTION_API_URL = 'https://goal-tracker-app.fly.dev';

/**
 * Start OAuth flow for mobile apps
 * Opens the browser with the OAuth URL and mobile=true parameter
 * so the backend knows to return a deep link instead of setting a cookie
 */
export async function startMobileOAuth(): Promise<void> {
  if (!Capacitor.isNativePlatform()) {
    console.warn('startMobileOAuth called on non-native platform');
    return;
  }

  const oauthUrl = `${PRODUCTION_API_URL}/api/v1/auth/google?mobile=true`;

  await Browser.open({ url: oauthUrl });
}
