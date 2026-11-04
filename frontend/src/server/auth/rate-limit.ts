import { rows, transaction } from "../db";
import { AuthError } from "./domain";
import { sha256 } from "./security";

// Shared in PostgreSQL across Next processes/restarts. Never trust arbitrary
// client-supplied forwarding headers. Account-specific lockout is additional.
export async function limitPublic(action: string, maximum: number, seconds: number) {
  const allowed = await transaction(async db => {
    const [v] = await rows<{ failures: number }>(db, `INSERT INTO auth.login_attempts(identifier_hash,failures) VALUES($1,1)
      ON CONFLICT(identifier_hash) DO UPDATE SET
      failures=CASE WHEN auth.login_attempts.window_started_at<clock_timestamp()-$2*interval '1 second' THEN 1 ELSE auth.login_attempts.failures+1 END,
      window_started_at=CASE WHEN auth.login_attempts.window_started_at<clock_timestamp()-$2*interval '1 second' THEN clock_timestamp() ELSE auth.login_attempts.window_started_at END
      RETURNING failures`, [sha256(`http:${action}`),seconds]);
    return v.failures <= maximum;
  });
  if (!allowed) throw new AuthError(429,"Too many attempts. Try again later.");
}
