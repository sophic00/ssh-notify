#!/bin/sh

# Configuration
TOKEN="YOUR_SERVER_TOKEN"
URL="http://YOUR_CENTRAL_BOT_IP_OR_DOMAIN:8080/ssh-login"

# Allow manual testing if PAM environment variables are not set
if [ -z "$PAM_TYPE" ]; then
    PAM_TYPE="open_session"
    PAM_USER="${PAM_USER:-$(whoami)}"
    PAM_RHOST="${PAM_RHOST:-"127.0.0.1"}"
fi

# Notify on session open (login)
if [ "$PAM_TYPE" = "open_session" ]; then
    SRV_HOSTNAME=$(hostname)
    TIME=$(date "+%Y-%m-%d %H:%M:%S %Z")

    # Send the login payload to the central notification server
    RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"$PAM_USER\", \"ip\":\"$PAM_RHOST\", \"hostname\":\"$SRV_HOSTNAME\", \"time\":\"$TIME\"}" \
        --max-time 5 \
        "$URL")

    if [ "$RESPONSE" != "200" ]; then
        logger -t ssh-telegram "Failed to send SSH login notification to central bot (HTTP status: $RESPONSE)"
    fi
fi
