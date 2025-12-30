import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Output static files for embedding in Go binary
  output: "export",
  // Disable image optimization for static export
  images: {
    unoptimized: true,
  },
  // Enable trailing slashes for better static file serving
  trailingSlash: true,
  // Proxy API requests to the Go backend during development
  async rewrites() {
    // Only apply rewrites in development mode
    // In production, the Go server serves both UI and API
    return process.env.NODE_ENV === "development"
      ? [
          {
            source: "/api/:path*",
            destination: "http://localhost:8080/api/:path*",
          },
        ]
      : [];
  },
};

export default nextConfig;
