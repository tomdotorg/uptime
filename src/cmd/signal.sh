ps | grep uptime | grep foo | cut -b 1-6 | xargs kill -HUP
