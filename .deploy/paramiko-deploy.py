import os
import posixpath
import sys
import time

import paramiko


HOST = "124.223.28.126"
USER = "root"
REMOTE_DIR = "/opt/ai-data-mvp"
REMOTE_ARCHIVE = "/tmp/ai-data-mvp-deploy.tar.gz"
SERVICE_PATH = "/etc/systemd/system/ai-data-mvp.service"


def connect():
    password = os.environ.get("DEPLOY_PASS")
    if not password:
        raise RuntimeError("DEPLOY_PASS is required")
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=password, timeout=20, banner_timeout=20, auth_timeout=20)
    return client


def run(client, command, check=True):
    stdin, stdout, stderr = client.exec_command(command, timeout=120)
    code = stdout.channel.recv_exit_status()
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    if out:
        print(out, end="")
    if err:
        print(err, end="", file=sys.stderr)
    if check and code != 0:
        raise RuntimeError(f"remote command failed ({code}): {command}")
    return code, out, err


def upload(client, local_path, remote_path):
    sftp = client.open_sftp()
    try:
        sftp.put(local_path, remote_path)
    finally:
        sftp.close()


def install_service(client):
    service = f"""[Unit]
Description=AI Data MVP Console
After=network.target

[Service]
Type=simple
WorkingDirectory={REMOTE_DIR}
Environment=PORT=3120
Environment=NODE_ENV=production
ExecStart=/usr/bin/npm run start:all
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
"""
    escaped = service.replace("\\", "\\\\").replace("$", "\\$").replace("`", "\\`")
    run(client, f"cat > {SERVICE_PATH} <<'EOF'\n{escaped}\nEOF")
    run(client, "systemctl daemon-reload")
    run(client, "systemctl enable ai-data-mvp.service")
    run(client, "systemctl restart ai-data-mvp.service")


def deploy():
    local_archive = os.path.abspath("ai-data-mvp-deploy.tar.gz")
    if not os.path.exists(local_archive):
        raise RuntimeError(f"missing archive: {local_archive}")

    client = connect()
    try:
        run(client, "node -v && npm -v")
        run(client, f"mkdir -p {REMOTE_DIR}")
        upload(client, local_archive, REMOTE_ARCHIVE)
        run(client, f"tar -xzf {REMOTE_ARCHIVE} -C {REMOTE_DIR}")
        install_service(client)
        time.sleep(2)
        run(client, "systemctl --no-pager --full status ai-data-mvp.service | sed -n '1,16p'", check=False)
        run(client, "curl -sS http://127.0.0.1:3120/api/scenarios | head -c 500")
    finally:
        client.close()


if __name__ == "__main__":
    deploy()
