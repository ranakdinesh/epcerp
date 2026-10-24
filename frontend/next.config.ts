import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  // Loopback-only QA origin; this option applies to the development server.
  allowedDevOrigins: ["127.0.0.1"],
};

export default nextConfig;
