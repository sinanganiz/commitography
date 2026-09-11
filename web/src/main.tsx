import { createRoot } from 'react-dom/client';

import { AppShell } from './app/AppShell';
import { mountLegacy, readEmbeddedReport } from './main';

function mount(): void {
  const root = document.getElementById('commitography-root');
  if (!root) return;

  if (readEmbeddedReport()) {
    mountLegacy();
    return;
  }

  createRoot(root).render(<AppShell />);
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', mount, { once: true });
} else {
  mount();
}
