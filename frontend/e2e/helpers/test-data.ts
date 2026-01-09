export const TEST_USERS = {
  tester: {
    email: process.env.TEST_USER_EMAIL || 'test@example.com',
  }
};

export const TEST_GOALS = {
  simple: {
    name: 'Read for 30 minutes',
    color: '#5B8C5A',
  },
  withWeeklyTarget: {
    name: 'Exercise',
    color: '#5B8C5A',
    targetCount: 3,
    targetPeriod: 'week' as const,
  },
  withMonthlyTarget: {
    name: 'Write',
    color: '#708090',
    targetCount: 20,
    targetPeriod: 'month' as const,
  },
};

export function generateTestGoalName(prefix: string = 'Test Goal'): string {
  return `${prefix} ${Date.now()}`;
}

export function getTodayDate(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
}

export function getCurrentMonth(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
}

export function getTodayDayNumber(): number {
  return new Date().getDate();
}
