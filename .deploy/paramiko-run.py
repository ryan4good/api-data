import os
import sys

import paramiko


HOST = "124.223.28.126"
USER = "root"


def main():
    password = os.environ.get("DEPLOY_PASS")
    if not password:
      raise RuntimeError("DEPLOY_PASS is required")
    command = " ".join(sys.argv[1:])
    if not command:
        raise RuntimeError("remote command is required")
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=password, timeout=20, banner_timeout=20, auth_timeout=20)
    try:
        stdin, stdout, stderr = client.exec_command(command, timeout=120)
        code = stdout.channel.recv_exit_status()
        out = stdout.read().decode("utf-8", errors="replace")
        err = stderr.read().decode("utf-8", errors="replace")
        if out:
            print(out, end="")
        if err:
            print(err, end="", file=sys.stderr)
        sys.exit(code)
    finally:
        client.close()


if __name__ == "__main__":
    main()
