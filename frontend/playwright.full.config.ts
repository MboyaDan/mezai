import { defineConfig, devices } from "@playwright/test";

// Full-stack tests: the real Go API, PostgreSQL with the sample seed and
// mailpit (cd backend && make up && make migrate && make seed). They create
// accounts, so they run one at a time. CI runs them in the "Full stack" job.
const port = Number(process.env.PORT ?? 3000);

export default defineConfig({
  testDir: "tests/full",
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  timeout: 90_000,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: { baseURL: `http://localhost:${port}`, trace: "retain-on-failure" },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
  ],
  webServer: {
    command: process.env.CI ? `npm run start -- -p ${port}` : `npm run dev -- -p ${port}`,
    url: `http://localhost:${port}`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
