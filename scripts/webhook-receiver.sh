#!/usr/bin/env bash
#
# Webhook Receiver for Local Testing
# ==================================
# This script starts a local HTTP server that receives and logs webhook alerts
# from CronJob Guardian when running with `make run`.
#
# Usage:
#   ./scripts/webhook-receiver.sh [PORT]
#
# Default port: 9090
#
# The server listens on all interfaces (0.0.0.0) and logs incoming alerts
# to stdout with pretty formatting.
#

set -euo pipefail

PORT="${1:-9090}"

# Check if Python is available
if ! command -v python3 &> /dev/null; then
    echo "Error: python3 is required but not installed"
    exit 1
fi

echo "=============================================================="
echo "  CRONJOB GUARDIAN - Webhook Receiver"
echo "=============================================================="
echo ""
echo "  Listening on: http://0.0.0.0:${PORT}"
echo "  Endpoint:     POST /alerts"
echo ""
echo "  Configure your AlertChannel secret with:"
echo "    url: http://host.docker.internal:${PORT}/alerts"
echo ""
echo "  Or if running operator outside of Docker:"
echo "    url: http://localhost:${PORT}/alerts"
echo ""
echo "=============================================================="
echo ""
echo "Waiting for alerts... (Press Ctrl+C to stop)"
echo ""

python3 -c "
import json
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
from datetime import datetime

# Force unbuffered output
sys.stdout.reconfigure(line_buffering=True)

class WebhookHandler(BaseHTTPRequestHandler):
    alert_count = 0

    def log_message(self, format, *args):
        pass  # Suppress default HTTP logging

    def do_POST(self):
        WebhookHandler.alert_count += 1
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length)

        now = datetime.now().strftime('%Y-%m-%d %H:%M:%S')

        print()
        print('=' * 70)
        print(f'ALERT #{WebhookHandler.alert_count} - {now}')
        print(f'Path: {self.path}')
        print('=' * 70)

        try:
            data = json.loads(body)

            # Extract key fields
            severity = data.get('severity', 'unknown').upper()
            alert_type = data.get('type', 'unknown')

            cronjob = data.get('cronjob', {})
            if isinstance(cronjob, dict):
                cj_ns = cronjob.get('namespace', '?')
                cj_name = cronjob.get('name', '?')
                cj_display = f'{cj_ns}/{cj_name}'
            else:
                cj_display = str(cronjob)

            title = data.get('title', 'N/A')
            message = data.get('message', 'N/A')

            # Color codes for terminal
            colors = {
                'CRITICAL': '\033[91m',  # Red
                'WARNING': '\033[93m',   # Yellow
                'INFO': '\033[94m',      # Blue
                'UNKNOWN': '\033[90m',   # Gray
            }
            reset = '\033[0m'
            color = colors.get(severity, colors['UNKNOWN'])

            print(f'  {color}[{severity}]{reset} {alert_type}')
            print(f'  CronJob:  {cj_display}')
            print(f'  Title:    {title}')
            print(f'  Message:  {message}')

            if data.get('suggestedFix'):
                print(f'  Fix:      {data[\"suggestedFix\"]}')

            if data.get('logs'):
                print()
                print('  --- Logs (last 10 lines) ---')
                log_lines = data['logs'].split('\\n')[-10:]
                for line in log_lines:
                    if line.strip():
                        print(f'    {line}')

            print()
            print('  Full payload:')
            print(json.dumps(data, indent=2))

        except json.JSONDecodeError:
            print('  (Raw body - not JSON)')
            print(body.decode('utf-8', errors='replace'))

        print('=' * 70)
        print()
        sys.stdout.flush()

        # Send success response
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(b'{\"status\": \"received\"}')

    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = json.dumps({
            'status': 'healthy',
            'alerts_received': WebhookHandler.alert_count
        })
        self.wfile.write(response.encode())

server = HTTPServer(('0.0.0.0', ${PORT}), WebhookHandler)
server.serve_forever()
"
