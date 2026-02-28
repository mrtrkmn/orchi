/** API response and request types for the Orchi CTF Platform */

export interface User {
  id: string;
  username: string;
  email?: string;
  role: 'admin' | 'organizer' | 'participant';
  team_id?: string;
}

export interface AuthResponse {
  user: User;
  token: string;
  refresh_token: string;
  expires_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface Event {
  id: string;
  name: string;
  type: string;
  status: 'running' | 'upcoming' | 'closed';
  start_time: string;
  end_time: string;
  team_count: number;
  max_teams: number;
  challenges_count: number;
  vpn_enabled: boolean;
  browser_access: boolean;
}

export interface EventListResponse {
  events: Event[];
  pagination: Pagination;
}

export interface Pagination {
  page: number;
  per_page: number;
  total: number;
}

export interface Team {
  id: string;
  name: string;
  score: number;
  rank: number;
  members_count: number;
  challenges_solved: number;
  last_solve_at?: string;
}

export interface Challenge {
  id: string;
  name: string;
  category: string;
  difficulty: 'easy' | 'medium' | 'hard' | 'insane';
  points: number;
  description: string;
  solved_by: number;
  solved: boolean;
  tags: string[];
  has_instance: boolean;
  instance_status?: string;
}

export interface ChallengeListResponse {
  challenges: Challenge[];
  categories: string[];
}

export interface FlagSubmitRequest {
  event_id: string;
  challenge_id: string;
  flag: string;
}

export interface FlagSubmitResponse {
  correct: boolean;
  points_awarded?: number;
  new_total_score?: number;
  new_rank?: number;
  first_blood?: boolean;
  message: string;
}

export interface TeamScore {
  rank: number;
  team_id: string;
  team_name: string;
  score: number;
  challenges_solved: number;
  last_solve_at?: string;
}

export interface ScoreboardResponse {
  event_id: string;
  last_updated: string;
  teams: TeamScore[];
  frozen: boolean;
}

export interface Lab {
  id: string;
  status: string;
  created_at: string;
  expires_at: string;
  exercises: Exercise[];
  frontend?: Frontend;
}

export interface Exercise {
  id: string;
  name: string;
  status: string;
  ip?: string;
}

export interface Frontend {
  type: string;
  status: string;
  rdp_url?: string;
}

export interface ScoreUpdateMessage {
  type: 'score_update';
  data: {
    team_id: string;
    team_name: string;
    new_score: number;
    new_rank: number;
    challenge_name: string;
    points: number;
    first_blood: boolean;
    timestamp: string;
  };
}

export interface APIError {
  error: {
    code: string;
    message: string;
    details?: unknown;
  };
}
