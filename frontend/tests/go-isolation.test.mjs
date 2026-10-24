import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { resolve, join } from "node:path";

const root = resolve(import.meta.dirname, "..");
function sourceFiles(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
    const path = join(dir, entry.name);
    return entry.isDirectory() ? sourceFiles(path) : /\.tsx?$/.test(path) ? [path] : [];
  });
}
test("Go frontend does not inherit Python upstream or session settings", () => {
  for (const path of sourceFiles(join(root, "src"))) {
    const source = readFileSync(path, "utf8");
    assert.doesNotMatch(source, /BACKEND_API_URL|localhost:9091|vtpl_access_token|vtpl_refresh_token|vtpl-session-renewal/, path);
  }
  const config = readFileSync(join(root, "src/lib/auth/backend-config.ts"), "utf8");
  assert.match(config, /import "server-only"/);
  assert.match(config, /EPCERP_API_URL/);
  assert.match(config, /127\.0\.0\.1:8001\/api\/v1/);
});
test("unmigrated workflow routes are unpublished", () => {
  for (const path of ["src/app/tenders", "src/app/demo", "src/app/api/tenders", "src/app/api/purchase-recovery"]) {
    assert.equal(existsSync(join(root, path)), false, path);
  }
});

test("Projects proxy publishes the independently implemented ownership API", () => {
 const route = readFileSync(join(root, "src/app/api/projects/[[...path]]/route.ts"), "utf8");
 assert.match(route, /proxyAuthenticatedRequest/);
 assert.match(route, /"projects"/);
 assert.match(route, /new Set\(\["GET", "POST", "PATCH"\]\)/);
});
