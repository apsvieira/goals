import { get } from 'svelte/store';
import { format } from 'svelte-i18n';

/**
 * Maps technical error messages to user-friendly messages.
 * Technical details are logged to console for debugging.
 */
export function getUserFriendlyMessage(error: unknown): string {
  let t: (id: string) => string;
  try {
    t = get(format);
  } catch {
    // i18n not yet initialised — fall back to raw message
    return error instanceof Error ? error.message : String(error);
  }
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
    // Return more specific message if available
    if (lowerMsg.includes('name')) return t('error.validationName');
    if (lowerMsg.includes('color')) return t('error.validationColor');
    if (lowerMsg.includes('date')) return t('error.validationDate');
    if (lowerMsg.includes('future')) return t('error.validationFuture');
    return t('error.validationGeneric');
  }

  // Conflict errors (e.g., duplicate)
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

  // Log unknown errors for debugging
  console.error('[App Error]', msg);
  return t('error.unknown');
}
