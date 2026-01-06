/**
 * Maps technical error messages to user-friendly messages.
 * Technical details are logged to console for debugging.
 */
export function getUserFriendlyMessage(error: unknown): string {
  const msg = error instanceof Error ? error.message : String(error);
  const lowerMsg = msg.toLowerCase();

  // Network errors
  if (msg.includes('Failed to fetch') || msg.includes('NetworkError') || lowerMsg.includes('network')) {
    console.error('[Network Error]', msg);
    return 'Unable to connect. Please check your internet connection.';
  }

  // Auth errors
  if (msg.includes('401') || lowerMsg.includes('unauthorized')) {
    console.error('[Auth Error]', msg);
    return 'Your session has expired. Please sign in again.';
  }

  // Rate limiting
  if (msg.includes('429') || lowerMsg.includes('rate limit')) {
    console.error('[Rate Limit]', msg);
    return 'Too many requests. Please wait a moment.';
  }

  // Not found errors
  if (msg.includes('404') || lowerMsg.includes('not found')) {
    console.error('[Not Found]', msg);
    return 'The item was not found. It may have been deleted.';
  }

  // Validation errors
  if (msg.includes('400') || lowerMsg.includes('invalid') || lowerMsg.includes('required')) {
    console.error('[Validation Error]', msg);
    // Return more specific message if available
    if (lowerMsg.includes('name')) return 'Please enter a valid goal name.';
    if (lowerMsg.includes('color')) return 'Please select a valid color.';
    if (lowerMsg.includes('date')) return 'Please select a valid date.';
    if (lowerMsg.includes('future')) return 'Cannot mark future dates as complete.';
    return 'Please check your input and try again.';
  }

  // Conflict errors (e.g., duplicate)
  if (msg.includes('409') || lowerMsg.includes('conflict') || lowerMsg.includes('already exists')) {
    console.error('[Conflict Error]', msg);
    return 'This item already exists.';
  }

  // Server errors
  if (msg.includes('500') || msg.includes('502') || msg.includes('503') || lowerMsg.includes('internal server')) {
    console.error('[Server Error]', msg);
    return 'Server error. Please try again later.';
  }

  // Sync errors
  if (lowerMsg.includes('sync')) {
    console.error('[Sync Error]', msg);
    return 'Unable to sync your data. Changes will be saved locally.';
  }

  // Log unknown errors for debugging
  console.error('[App Error]', msg);
  return 'Something went wrong. Please try again.';
}
