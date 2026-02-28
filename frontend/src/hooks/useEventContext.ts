import { useEffect, useState } from 'react';
import api from '../lib/api';
import type { Event } from '../types/api';

/**
 * Reserved subdomains that are NOT event slugs.
 */
const RESERVED_SUBDOMAINS = new Set([
  'api',
  'desktop',
  'staging',
  'www',
  'admin',
  'grafana',
  'monitoring',
]);

/**
 * Extract the event slug from the current hostname.
 *
 * Examples:
 *   ctf2026.cyberorch.com → "ctf2026"
 *   cyberorch.com          → null
 *   api.cyberorch.com      → null (reserved)
 *   localhost               → null
 */
export function getEventSlug(): string | null {
  const host = window.location.hostname;

  // Local development — no subdomain detection
  if (host === 'localhost' || /^\d+\.\d+\.\d+\.\d+$/.test(host)) {
    // Allow ?event=slug query param for local testing
    const params = new URLSearchParams(window.location.search);
    return params.get('event');
  }

  const parts = host.split('.');
  // Must have at least 3 parts: sub.domain.tld
  if (parts.length < 3) return null;

  const subdomain = parts[0].toLowerCase();
  if (RESERVED_SUBDOMAINS.has(subdomain)) return null;

  return subdomain;
}

interface EventContextState {
  /** Event slug extracted from subdomain (e.g. "ctf2026") */
  slug: string | null;
  /** Full event data from API (null while loading or if no event) */
  event: Event | null;
  /** Loading state */
  isLoading: boolean;
  /** Error message if event slug doesn't match a real event */
  error: string | null;
  /** Whether the current page is in event context (has a valid slug) */
  isEventContext: boolean;
}

/**
 * Hook to detect and load event context from the subdomain.
 *
 * Usage in components:
 *   const { event, isEventContext, isLoading } = useEventContext();
 *   if (isEventContext && event) { ... }
 */
export function useEventContext(): EventContextState {
  const slug = getEventSlug();
  const [event, setEvent] = useState<Event | null>(null);
  const [isLoading, setIsLoading] = useState(!!slug);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!slug) {
      setIsLoading(false);
      return;
    }

    let cancelled = false;

    const fetchEvent = async () => {
      setIsLoading(true);
      setError(null);
      try {
        const res = await api.get<Event>(`/events/by-slug/${slug}`);
        if (!cancelled) {
          setEvent(res.data);
          setIsLoading(false);
        }
      } catch (err: unknown) {
        if (!cancelled) {
          setError(`Event "${slug}" not found`);
          setIsLoading(false);
        }
      }
    };

    fetchEvent();

    return () => {
      cancelled = true;
    };
  }, [slug]);

  return {
    slug,
    event,
    isLoading,
    error,
    isEventContext: !!slug,
  };
}
