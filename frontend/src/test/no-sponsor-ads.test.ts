import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

const frontendRoot = resolve(process.cwd());

function source(path: string) {
  return readFileSync(resolve(frontendRoot, path), 'utf8');
}

describe('user-facing pages', () => {
  it('do not expose sponsor or donation advertisements', () => {
    expect(source('src/pages/index/IndexPage.tsx')).not.toMatch(/SponsorSlot|sponsor/i);
    expect(source('src/pages/login/LoginPage.tsx')).not.toMatch(/SponsorSlot|sponsor/i);
    expect(source('src/layouts/AppSidebar.tsx')).not.toMatch(
      /DonateButton|DONATE_URL|donate\.sanaei\.dev|sidebar-donate/i,
    );
    expect(source('src/layouts/AppSidebar.css')).not.toMatch(/sidebar-donate/);
    expect(source('src/pages/login/LoginPage.css')).not.toMatch(/login-sponsor/);
    expect(source('src/routes/panelRoutes.tsx')).not.toMatch(
      /SponsorsPage|\/sponsors|menu\.sponsors/,
    );
  });
});
