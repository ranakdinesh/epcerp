import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  outputFileTracingIncludes: { "/*": ["./src/server/purchasing/documents/assets/**/*", "./src/server/purchasing/loi-documents/assets/**/*"] },
  // QA hosts resolve to loopback; this option applies only to the development server.
  allowedDevOrigins: ["127.0.0.1", "epcerp.test"],
};

export default nextConfig;
