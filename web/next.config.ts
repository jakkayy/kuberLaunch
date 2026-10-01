import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Self-contained server for the container image (see web/Dockerfile).
  output: "standalone",
};

export default nextConfig;
