import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Output static files for embedding in Go binary
  output: "export",
  // Disable image optimization for static export
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
