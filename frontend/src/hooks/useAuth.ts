import { create } from 'zustand';
import api from '../lib/api';
import type { User, LoginRequest, RegisterRequest, AuthResponse } from '../types/api';

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;

  login: (credentials: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => void;
  loadFromStorage: () => void;
}

/**
 * Authentication store using Zustand.
 *
 * Manages JWT tokens, user state, and auth operations.
 * Tokens are persisted in localStorage for session continuity.
 */
export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  token: null,
  isAuthenticated: false,
  isLoading: false,
  error: null,

  login: async (credentials: LoginRequest) => {
    set({ isLoading: true, error: null });
    try {
      const response = await api.post<AuthResponse>('/auth/login', credentials);
      const { user, token, refresh_token } = response.data;

      localStorage.setItem('orchi_token', token);
      localStorage.setItem('orchi_refresh_token', refresh_token);
      localStorage.setItem('orchi_user', JSON.stringify(user));

      set({
        user,
        token,
        isAuthenticated: true,
        isLoading: false,
      });
    } catch (err: unknown) {
      const message = (err as { response?: { data?: { error?: { message?: string } } } })
        ?.response?.data?.error?.message || 'Login failed';
      set({ error: message, isLoading: false });
    }
  },

  register: async (data: RegisterRequest) => {
    set({ isLoading: true, error: null });
    try {
      const response = await api.post<AuthResponse>('/auth/register', data);
      const { user, token, refresh_token } = response.data;

      localStorage.setItem('orchi_token', token);
      localStorage.setItem('orchi_refresh_token', refresh_token);
      localStorage.setItem('orchi_user', JSON.stringify(user));

      set({
        user,
        token,
        isAuthenticated: true,
        isLoading: false,
      });
    } catch (err: unknown) {
      const message = (err as { response?: { data?: { error?: { message?: string } } } })
        ?.response?.data?.error?.message || 'Registration failed';
      set({ error: message, isLoading: false });
    }
  },

  logout: () => {
    localStorage.removeItem('orchi_token');
    localStorage.removeItem('orchi_refresh_token');
    localStorage.removeItem('orchi_user');

    set({
      user: null,
      token: null,
      isAuthenticated: false,
    });
  },

  loadFromStorage: () => {
    const token = localStorage.getItem('orchi_token');
    const userStr = localStorage.getItem('orchi_user');

    if (token && userStr) {
      try {
        const user = JSON.parse(userStr) as User;
        set({ user, token, isAuthenticated: true });
      } catch {
        // Corrupted storage, clear it
        localStorage.removeItem('orchi_token');
        localStorage.removeItem('orchi_refresh_token');
        localStorage.removeItem('orchi_user');
      }
    }
  },
}));
