// Server-only configuration: tokens and the upstream address never enter client props.
import "server-only";

export const backendApiUrl = process.env.EPCERP_API_URL ?? "http://127.0.0.1:8001/api/v1";

const endpoint = new URL(backendApiUrl);
if (
  !["http:", "https:"].includes(endpoint.protocol) ||
  endpoint.username || endpoint.password || endpoint.search || endpoint.hash ||
  endpoint.pathname !== "/api/v1" ||
  (process.env.NODE_ENV === "production" && !process.env.EPCERP_API_URL)
) {
  throw new Error("Configure EPCERP_API_URL with the independent Go API address ending in /api/v1");
}
