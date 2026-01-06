/**
 * Maps technical error messages to user-friendly messages.
 * Technical details are logged to console for debugging.
 */
export function getUserFriendlyMessage(error: unknown): string {
  const msg = error instanceof Error ? error.message : String(error);

  // Network errors
  if (msg.includes('Failed to fetch') || msg.includes('NetworkError') || msg.includes('network')) {
    console.error('[Network Error]', msg);
    return 'Unable to connect. Please check your internet connection.';
  }

  // Auth errors
  if (msg.includes('401') || msg.toLowerCase().includes('unauthorized')) {
    console.error('[Auth Error]', msg);
    return 'Your session has expired. Please sign in again.';
  }

  // Rate limiting
  if (msg.includes('429') || msg.toLowerCase().includes('rate limit')) {
    console.error('[Rate Limit]', msg);
    return 'Too many requests. Please wait a moment.';
  }

  // Server errors
  if (msg.includes('500') || msg.toLowerCase().includes('internal server')) {
    console.error('[Server Error]', msg);
    return 'Server error. Please try again later.';
  }

  // Log unknown errors for debugging
  console.error('[App Error]', msg);
  return 'Something went wrong. Please try again.';
}
