import bcrypt from "bcryptjs";

// A valid bcrypt hash of a throwaway string. Used so that a bcrypt compare
// always runs even when the username is wrong or ADMIN_PASSWORD_HASH is
// unset, keeping response timing uniform. Its result is never trusted.
const DUMMY_HASH = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy";

/**
 * Verify the single admin account: username must equal ADMIN_USERNAME
 * (default "admin") and the password must match ADMIN_PASSWORD_HASH (bcrypt).
 */
export async function verifyCredentials(
  username: string,
  password: string,
): Promise<boolean> {
  const expectedUsername = process.env.ADMIN_USERNAME || "admin";
  const passwordHash = process.env.ADMIN_PASSWORD_HASH;

  const usernameOk = username === expectedUsername;
  const hashToCheck = usernameOk && passwordHash ? passwordHash : DUMMY_HASH;
  const passwordOk = await bcrypt.compare(password, hashToCheck);

  return usernameOk && Boolean(passwordHash) && passwordOk;
}
