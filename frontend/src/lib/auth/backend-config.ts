// Server-only configuration: tokens and the upstream address never enter client props.
import "server-only";

export const backendApiUrl = process.env.EPCERP_API_URL ?? "http://127.0.0.1:1/api/v1";

const endpoint = new URL(backendApiUrl);
if (
  !["http:", "https:"].includes(endpoint.protocol) ||
  endpoint.username || endpoint.password || endpoint.search || endpoint.hash ||
  endpoint.pathname !== "/api/v1"
) {
  throw new Error("Configure EPCERP_API_URL with the internal route namespace ending in /api/v1");
}
