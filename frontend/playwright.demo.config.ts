import { defineConfig, devices } from "@playwright/test";

// The pilot demo script (brief section 18) against a deployed site. Run it
// through `backend/scripts/staging.sh demo`, which sets DEMO_BASE_URL,
// DEMO_DB_URL and SEED_PASSWORD. It never starts a server of its own.
export default defineConfig({
  testDir: "tests/demo",
  workers: 1,
  retries: 0,
  timeout: 120_000,
  reporter: "list",
  use: {
    baseURL: process.env.DEMO_BASE_URL ?? "http://localhost:3000",
    navigationTimeout: 90_000,
    actionTimeout: 30_000,
    trace: "retain-on-failure",
  },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
  ],
});
