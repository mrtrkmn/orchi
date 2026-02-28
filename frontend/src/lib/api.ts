import axios from 'axios';

/**
 * Determine the API base URL.
 *
 * - localhost / IP  → use /api/v1 (vite proxy or port-forward)
 * - *.cyberorch.com → api.cyberorch.com/api/v1
 */
function resolveApiUrl(): string {
  if (import.meta.env.VITE_API_URL) return import.meta.env.VITE_API_URL;

  const host = window.location.hostname;

  // Local development
  if (host === 'localhost' || /^\d+\.\d+\.\d+\.\d+$/.test(host)) {
    return '/api/v1';
  }

  // Production: always route to api.cyberorch.com regardless of subdomain
  const rootDomain = host.split('.').slice(-2).join('.');
  return `${window.location.protocol}//api.${rootDomain}/api/v1`;
}

const API_BASE_URL = resolveApiUrl();

/**
 * Axios instance configured for the Orchi API.
 *
 * - Automatically attaches JWT token from localStorage
 * - Handles 401 responses by redirecting to login
 * - Sets JSON content type
 */
const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor: attach JWT token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('orchi_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor: handle auth errors
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      // Token expired or invalid — clear and redirect to login
      localStorage.removeItem('orchi_token');
      localStorage.removeItem('orchi_refresh_token');
      localStorage.removeItem('orchi_user');

      // Only redirect if not already on login/register page
      if (
        !window.location.pathname.includes('/login') &&
        !window.location.pathname.includes('/register')
      ) {
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  }
);

export default api;
