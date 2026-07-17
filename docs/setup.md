# System Setup Guide

This document describes how to deploy the SSH Notify Bot daemon and configure client servers for multi-server monitoring.

---

## 1. Prerequisites

- A Go compiler (version 1.26.5 or later) installed on the system where the central bot will run.
- SQLite support on the central host (automatically handled by the pure Go SQLite driver).
- `curl` installed on all client servers to send HTTP POST requests.
- A Telegram bot token created via BotFather.

---

## 2. Obtain Your Telegram User ID

To secure the bot, you must configure the owner's Telegram User ID. Only messages from this ID will be processed.

To find your Telegram User ID:
1. Open Telegram and search for `@userinfobot` or `@RawDataBot`.
2. Start the bot. It will respond with your numeric User ID (e.g., `123456789`).
3. Note this ID to use as `OWNER_USER_ID`.

---

## 3. Central Daemon Deployment

Follow these steps to build and run the daemon on your central server.

### Build from Source

Navigate to the project root directory and compile the binary:

```bash
go build -o ssh-notify-bot .
```

### Configuration

The daemon is configured using environment variables. Create a configuration file at `/etc/ssh-notify.env` containing:

```env
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
OWNER_USER_ID=your_numeric_user_id_here
HTTP_ADDR=:8080
DATABASE_PATH=/var/lib/ssh-notify/ssh_notify.db
```

Ensure the directory `/var/lib/ssh-notify` exists and is writable by the user running the service.

### Systemd Service Setup

To manage the daemon as a system service, create a unit file at `/etc/systemd/system/ssh-notify.service`:

```ini
[Unit]
Description=SSH Notification Telegram Bot Daemon
After=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
EnvironmentFile=/etc/ssh-notify.env
ExecStart=/usr/local/bin/ssh-notify-bot
Restart=always
RestartSec=5

# Simple sandboxing options
StateDirectory=ssh-notify
ReadWritePaths=/var/lib/ssh-notify

[Install]
WantedBy=multi-user.target
```

Ensure you copy the compiled binary to `/usr/local/bin/ssh-notify-bot`:

```bash
sudo cp ssh-notify-bot /usr/local/bin/
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable ssh-notify
sudo systemctl start ssh-notify
```

Verify the status of the service:

```bash
sudo systemctl status ssh-notify
```

---

## 4. Bot Configuration and Chat Authorization

Once the daemon is running, open the Telegram chat with your bot.

1. Send the command `/start` to verify the bot is online.
2. If you want notifications sent to a personal chat, send the `/authchat` command in your private message thread with the bot.
3. If you want notifications sent to a group or channel:
   - Add the bot to that group/channel.
   - Run `/authchat` in the group/channel.
4. Use `/listchats` to verify the list of authorized destinations.

---

## 5. Adding and Registering Servers

For each server you wish to monitor, you must generate a unique access token.

1. In the bot private chat, run:
   ```text
   /addserver <server-identifier>
   ```
   Example: `/addserver web-prod-1`
2. The bot will save the server in the SQLite database and respond with a unique 32-character hexadecimal token.
3. Keep this token secure. You will need it to configure the client script on that server.

---

## 6. Client Server Installation (sshd integration)

Repeat these steps on each server you want to monitor.

### Step A: Place the Script

Copy the `ssh-telegram.sh` script to `/usr/local/bin/ssh-telegram.sh` on the target server.

### Step B: Configure the Script

Edit `/usr/local/bin/ssh-telegram.sh` and set the following parameters:
- `TOKEN`: The unique token generated in Step 5 for this server.
- `URL`: The public or internal address of your central daemon HTTP endpoint.
  Example: `http://192.168.1.50:8080/ssh-login`

Make the script executable:

```bash
sudo chmod +x /usr/local/bin/ssh-telegram.sh
```

### Step C: Configure PAM Trigger

To execute the script automatically whenever an SSH session is initialized, edit `/etc/pam.d/sshd` and append the following line:

```text
session optional pam_exec.so default=0 /usr/local/bin/ssh-telegram.sh
```

> [!NOTE]
> We use `optional` so that if the notification fails or the central daemon is offline, it does not prevent users from logging into the server.

### Step D: Test the Connection

You can manually execute the script to verify the connection works:

```bash
/usr/local/bin/ssh-telegram.sh
```

This will trigger a test payload using the current user and local loopback address, sending it to the central server. Check your authorized Telegram chats for the corresponding notification.
