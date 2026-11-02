import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  // QA hosts resolve to loopback; this option applies only to the development server.
  allowedDevOrigins: ["127.0.0.1", "epcerp.test"],
};

export default nextConfig;
