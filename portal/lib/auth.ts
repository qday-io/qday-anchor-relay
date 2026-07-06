import bcrypt from "bcryptjs";

// A valid bcrypt hash (cost 12, matching the real admin hash policy) of a
// throwaway string. Used so that a bcrypt compare always runs — at the same
// cost — even when the username is wrong or ADMIN_PASSWORD_HASH is unset,
// keeping response timing uniform. Its result is never trusted.
const DUMMY_HASH = "$2b$12$KZlLm1iZwpKj4AdMOWSpmOjv4wXMkUyXE9nOsLvzLMWzMQQLnLW52";

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
