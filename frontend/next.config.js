/** @type {import('next').NextConfig} */
const nextConfig = {
  // Static export: produces plain HTML/JS/CSS in out/ for embedding into the Go binary.
  // `next dev` ignores this setting; rewrites below still work during development.
  output: 'export',

  // Proxy API calls to the Go backend in development
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://localhost:8080/api/:path*',
      },
      {
        source: '/healthz',
        destination: 'http://localhost:8080/healthz',
      },
    ];
  },
};

module.exports = nextConfig;
