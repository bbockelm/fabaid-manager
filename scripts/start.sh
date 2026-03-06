#!/bin/sh
set -e

echo "Starting FabAID Manager..."

# Start the Go backend
/app/fabaid-server &

# Start the Next.js frontend (production mode)
cd /app
npx next start -p 3000 &

# Wait for either process to exit
wait -n
