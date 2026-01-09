import { APIRequestContext } from '@playwright/test';

export class GoalTrackerAPI {
  private request: APIRequestContext;
  private baseURL: string;

  constructor(request: APIRequestContext) {
    this.request = request;
    this.baseURL = 'http://localhost:8080/api/v1';
  }

  async createGoal(name: string, color: string, targetCount?: number, targetPeriod?: 'week' | 'month') {
    const response = await this.request.post(`${this.baseURL}/goals`, {
      data: {
        name,
        color,
        target_count: targetCount,
        target_period: targetPeriod,
      }
    });
    return response.json();
  }

  async getGoals() {
    const response = await this.request.get(`${this.baseURL}/goals`);
    return response.json();
  }

  async getCalendar(month: string) {
    const response = await this.request.get(`${this.baseURL}/calendar/${month}`);
    return response.json();
  }

  async createCompletion(goalId: string, date: string) {
    const response = await this.request.post(`${this.baseURL}/goals/${goalId}/completions`, {
      data: { date }
    });
    return response.json();
  }

  async deleteCompletion(goalId: string, date: string) {
    await this.request.delete(`${this.baseURL}/goals/${goalId}/completions/${date}`);
  }

  async deleteGoal(goalId: string) {
    await this.request.delete(`${this.baseURL}/goals/${goalId}`);
  }

  async archiveGoal(goalId: string) {
    await this.request.post(`${this.baseURL}/goals/${goalId}/archive`);
  }
}
