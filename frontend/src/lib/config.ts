import { Capacitor } from '@capacitor/core';

export const PRODUCTION_API_URL = 'https://goal-tracker-app.fly.dev';

export function getApiBase(): string {
  if (Capacitor.isNativePlatform()) {
    return `${PRODUCTION_API_URL}/api/v1`;
  }
  return '/api/v1';
}
