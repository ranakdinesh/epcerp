export function authCookieOptions(environment: {
  NODE_ENV?: string;
  EPCERP_ENVIRONMENT?: string;
  EPCERP_DEVELOPMENT_HTTP?: string;
}) {
  const developmentHttp = environment.EPCERP_DEVELOPMENT_HTTP === "true";
  if (developmentHttp && environment.EPCERP_ENVIRONMENT !== "development") {
    throw new Error("HTTP authentication is permitted only in the development environment");
  }
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: environment.NODE_ENV === "production" && !developmentHttp,
  };
}
