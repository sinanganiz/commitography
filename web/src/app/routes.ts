import { useEffect, useState } from 'react';

/**
 * The application's locations. They live in the URL hash because the server
 * only serves `/`: a hash survives a refresh, works with back and forward, and
 * is never sent to the server.
 */
export type Route = { name: 'analyze' } | { name: 'recent' } | { name: 'job'; id: string };

const JOB_ID = /^[0-9a-f]{8,64}$/;

export function parseRoute(hash: string): Route {
  const path = hash.replace(/^#\/?/, '');
  if (path === 'recent') return { name: 'recent' };
  const job = /^jobs\/([^/]+)$/.exec(path);
  if (job && JOB_ID.test(job[1])) return { name: 'job', id: job[1] };
  return { name: 'analyze' };
}

export function routeHash(route: Route): string {
  switch (route.name) {
    case 'analyze':
      return '#/';
    case 'recent':
      return '#/recent';
    case 'job':
      return `#/jobs/${route.id}`;
  }
}

/** Moves to a route and records it in the browser history. */
export function navigate(route: Route): void {
  const hash = routeHash(route);
  if (window.location.hash !== hash) window.location.hash = hash;
}

/** Tracks the current route, including back and forward navigation. */
export function useRoute(): Route {
  const [route, setRoute] = useState<Route>(() => parseRoute(window.location.hash));
  useEffect(() => {
    const update = () => setRoute(parseRoute(window.location.hash));
    window.addEventListener('hashchange', update);
    return () => window.removeEventListener('hashchange', update);
  }, []);
  return route;
}
