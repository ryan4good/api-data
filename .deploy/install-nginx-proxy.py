import os
from io import BytesIO

import paramiko


HOST = "124.223.28.126"
USER = "root"
SITE_PATH = "/etc/nginx/sites-enabled/stock-analyzer"
MARKER = "location /ai-data/"
BLOCK = """    location /ai-data/ {
        proxy_pass http://127.0.0.1:3120/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 120s;
    }

"""


def run(client, command):
    stdin, stdout, stderr = client.exec_command(command, timeout=60)
    code = stdout.channel.recv_exit_status()
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    if out:
        print(out, end="")
    if err:
        print(err, end="")
    if code != 0:
        raise RuntimeError(f"remote command failed ({code}): {command}")


def main():
    password = os.environ.get("DEPLOY_PASS")
    if not password:
        raise RuntimeError("DEPLOY_PASS is required")

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=password, timeout=20, banner_timeout=20, auth_timeout=20)
    try:
        sftp = client.open_sftp()
        try:
            with sftp.open(SITE_PATH, "r") as remote_file:
                content = remote_file.read().decode("utf-8")
            if MARKER not in content:
                insert_at = content.find("    location / {")
                if insert_at < 0:
                    raise RuntimeError("could not find root location block")
                updated = content[:insert_at] + BLOCK + content[insert_at:]
                with sftp.open(f"{SITE_PATH}.bak.ai-data", "w") as backup:
                    backup.write(content)
                with sftp.open(SITE_PATH, "w") as remote_file:
                    remote_file.write(updated)
        finally:
            sftp.close()

        run(client, "nginx -t")
        run(client, "systemctl reload nginx")
        run(client, "curl -sS http://127.0.0.1:8080/ai-data/api/scenarios | head -c 500")
    finally:
        client.close()


if __name__ == "__main__":
    main()
