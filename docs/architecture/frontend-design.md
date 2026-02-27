# Frontend Design — Modern CTF Platform

## Overview

The frontend is a standalone React Single Page Application (SPA) with a dark-mode
cybersecurity aesthetic. It communicates exclusively with the backend via REST APIs
and WebSocket connections. It is deployed as static assets in a container and is
independently scalable from the backend.

---

## Technology Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Framework | React 18 + TypeScript | Industry standard, strong ecosystem |
| Build Tool | Vite | Fast HMR, optimized builds |
| Routing | React Router v6 | Standard SPA routing |
| State | Zustand | Lightweight, no boilerplate |
| Server State | TanStack Query (React Query) | Caching, refetch, optimistic updates |
| Styling | Tailwind CSS | Utility-first, dark mode native |
| Components | shadcn/ui | Accessible, customizable |
| API Client | Axios | Interceptors for JWT refresh |
| WebSocket | Native WebSocket | Live scoreboard |
| Charts | Recharts | Score timeline visualization |
| Forms | React Hook Form + Zod | Validation, performance |
| Icons | Lucide React | Consistent icon set |
| Testing | Vitest + Testing Library | Fast, React-native testing |

---

## Design System

### Color Palette (Dark Mode)
```
Background:     #0a0a0f (near-black)
Surface:        #12121a (card backgrounds)
Surface Hover:  #1a1a2e (interactive elements)
Border:         #2a2a3e (subtle borders)
Primary:        #00ff88 (cyber green — CTF accent)
Secondary:      #6366f1 (indigo — secondary actions)
Danger:         #ef4444 (red — errors, wrong flags)
Success:        #22c55e (green — correct flags)
Warning:        #f59e0b (amber — time warnings)
Text Primary:   #e2e8f0 (light text)
Text Secondary: #94a3b8 (muted text)
Text Accent:    #00ff88 (highlighted text)
```

### Typography
```
Font Family:    'JetBrains Mono' (monospace — code/hacker aesthetic)
                'Inter' (sans-serif — body text)
Headings:       JetBrains Mono, bold
Body:           Inter, regular
Code/Flags:     JetBrains Mono, regular
```

---

## Routing Structure

```
/                           Landing page (public)
/login                      Login page (public)
/register                   Registration page (public)
/dashboard                  User dashboard (authenticated)
/events                     Event listing (authenticated)
/events/:eventId            Event detail / join
/events/:eventId/challenges Challenge browser
/events/:eventId/challenges/:id  Challenge detail + flag submit
/events/:eventId/scoreboard Live scoreboard
/events/:eventId/teams      Team listing
/teams/:teamId              Team detail
/lab                        Lab access panel
/lab/vpn                    VPN configuration
/admin                      Admin dashboard (admin only)
/admin/events               Event management
/admin/users                User management
/admin/challenges           Challenge management
```

---

## Component Tree

```
App
├── Layout
│   ├── Navbar
│   │   ├── Logo
│   │   ├── NavLinks
│   │   ├── EventSelector
│   │   └── UserMenu (avatar, logout)
│   ├── Sidebar (admin only)
│   └── Footer
│
├── Pages
│   ├── LandingPage
│   │   ├── HeroSection
│   │   ├── FeaturesGrid
│   │   └── CTASection
│   │
│   ├── AuthPages
│   │   ├── LoginPage
│   │   │   ├── LoginForm
│   │   │   └── SocialLogin (optional)
│   │   └── RegisterPage
│   │       ├── RegisterForm
│   │       └── PasswordStrength
│   │
│   ├── DashboardPage
│   │   ├── ActiveEventCard
│   │   ├── TeamOverview
│   │   ├── RecentSolves
│   │   └── QuickStats
│   │
│   ├── ChallengesPage
│   │   ├── CategoryFilter
│   │   ├── DifficultyFilter
│   │   ├── SearchBar
│   │   └── ChallengeGrid
│   │       └── ChallengeCard
│   │           ├── CategoryBadge
│   │           ├── DifficultyBadge
│   │           ├── PointsDisplay
│   │           └── SolvedIndicator
│   │
│   ├── ChallengeDetailPage
│   │   ├── ChallengeHeader
│   │   ├── ChallengeDescription (markdown)
│   │   ├── FlagSubmissionForm
│   │   ├── HintSection (expandable)
│   │   ├── SolvedByList
│   │   └── InstanceControls (start/stop/reset)
│   │
│   ├── ScoreboardPage
│   │   ├── ScoreboardHeader (event info, timer)
│   │   ├── TopThreeDisplay (podium)
│   │   ├── ScoreTable
│   │   │   ├── RankColumn
│   │   │   ├── TeamColumn
│   │   │   ├── ScoreColumn
│   │   │   └── SolvesColumn
│   │   ├── ScoreTimeline (chart)
│   │   └── LiveFeed (recent solves)
│   │
│   ├── LabPage
│   │   ├── LabStatusPanel
│   │   ├── ExerciseList
│   │   │   └── ExerciseCard (status, IP, reset button)
│   │   ├── VPNConfigPanel
│   │   │   ├── DownloadButton
│   │   │   └── ConnectionInstructions
│   │   └── BrowserAccessPanel
│   │       └── DesktopViewer (noVNC WebSocket)
│   │
│   ├── TeamPage
│   │   ├── TeamInfo
│   │   ├── MemberList
│   │   ├── TeamSolveHistory
│   │   └── TeamScoreChart
│   │
│   └── AdminPages
│       ├── AdminDashboard
│       │   ├── SystemStats
│       │   ├── ActiveEvents
│       │   └── RecentActivity
│       ├── EventManagement
│       │   ├── EventCreateForm
│       │   ├── EventList
│       │   └── EventDetail
│       ├── UserManagement
│       └── ChallengeManagement
│
└── Shared Components
    ├── LoadingSpinner
    ├── ErrorBoundary
    ├── Toast / Notification
    ├── ConfirmDialog
    ├── Countdown Timer
    ├── MarkdownRenderer
    └── ProtectedRoute
```

---

## State Management

### Zustand Stores

```typescript
// Auth Store
interface AuthStore {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  login: (credentials: LoginRequest) => Promise<void>;
  logout: () => void;
  refreshToken: () => Promise<void>;
}

// Event Store
interface EventStore {
  currentEvent: Event | null;
  events: Event[];
  setCurrentEvent: (event: Event) => void;
  fetchEvents: () => Promise<void>;
}

// Scoreboard Store (WebSocket-driven)
interface ScoreboardStore {
  teams: TeamScore[];
  lastUpdate: Date;
  frozen: boolean;
  connect: (eventId: string) => void;
  disconnect: () => void;
}
```

### React Query Keys
```typescript
const queryKeys = {
  events: ['events'],
  event: (id: string) => ['events', id],
  challenges: (eventId: string) => ['events', eventId, 'challenges'],
  challenge: (eventId: string, id: string) => ['events', eventId, 'challenges', id],
  scoreboard: (eventId: string) => ['events', eventId, 'scoreboard'],
  teams: (eventId: string) => ['events', eventId, 'teams'],
  team: (id: string) => ['teams', id],
  lab: (teamId: string) => ['teams', teamId, 'lab'],
};
```

---

## API Interaction Model

### Axios Instance with JWT Interceptors
```typescript
const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL,
  headers: { 'Content-Type': 'application/json' },
});

// Request interceptor — attach token
api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor — handle 401, refresh token
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401 && !error.config._retry) {
      error.config._retry = true;
      await useAuthStore.getState().refreshToken();
      return api(error.config);
    }
    return Promise.reject(error);
  }
);
```

---

## Real-Time Updates (WebSocket)

```typescript
function useScoreboardWebSocket(eventId: string) {
  const updateScoreboard = useScoreboardStore((s) => s.updateTeam);

  useEffect(() => {
    const ws = new WebSocket(
      `${WS_URL}/api/v1/ws/scoreboard?event_id=${eventId}`
    );

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data.type === 'score_update') {
        updateScoreboard(data.data);
      }
    };

    ws.onclose = () => {
      // Reconnect with exponential backoff
      setTimeout(() => connect(), backoff);
    };

    return () => ws.close();
  }, [eventId]);
}
```

---

## Role-Based UI Rendering

```typescript
function ProtectedRoute({ children, requiredRole }: Props) {
  const { user, isAuthenticated } = useAuthStore();

  if (!isAuthenticated) return <Navigate to="/login" />;
  if (requiredRole && user?.role !== requiredRole) return <Navigate to="/dashboard" />;

  return children;
}

// Usage in router
<Route path="/admin/*" element={
  <ProtectedRoute requiredRole="admin">
    <AdminLayout />
  </ProtectedRoute>
} />
```

---

## Mobile Responsiveness

- **Breakpoints**: Tailwind defaults (sm: 640px, md: 768px, lg: 1024px, xl: 1280px)
- **Challenge Grid**: 1 col mobile → 2 col tablet → 3 col desktop
- **Scoreboard**: Horizontal scroll on mobile, full table on desktop
- **Navigation**: Hamburger menu on mobile, full navbar on desktop
- **Lab Panel**: Stacked cards on mobile, side-by-side on desktop

---

## Deployment Strategy

### Container Image
```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

### Nginx Configuration
```nginx
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    # SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-Content-Type-Options "nosniff";
    add_header X-XSS-Protection "1; mode=block";
    add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' wss: https:;";
}
```

### Kubernetes Deployment
- Deploy as `Deployment` in `orchi-frontend` namespace
- Serve via Ingress at `orchi.io` / `app.orchi.io`
- HPA based on CPU utilization
- Optional CDN (CloudFront/Cloudflare) in front
