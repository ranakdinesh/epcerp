import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  outputFileTracingIncludes: { "/*": ["./src/server/purchasing/documents/assets/**/*", "./src/server/purchasing/loi-documents/assets/**/*"] },
  // Proxy otherwise silently truncates uploads at 10 MiB, before route validation.
  // Tender ZIP uploads allow 64 MiB; individual routes retain their own limits.
  experimental: { cpus: 2, proxyClientMaxBodySize: 64 * 1024 * 1024 },
  // QA hosts resolve to loopback; this option applies only to the development server.
  allowedDevOrigins: ["127.0.0.1", "epcerp.test"],
};

export default nextConfig;
