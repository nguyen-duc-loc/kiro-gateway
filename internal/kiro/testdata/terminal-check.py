"""Exercise only the local terminal fixture in a compiled Go test binary."""
import os
import pty
import select
import signal
import sys
import subprocess
import time

pid, master = pty.fork()
if pid == 0:
    binary = os.path.abspath(sys.argv[1])
    # Match the execution channel: terminal stdin with captured stdout/stderr.
    child = subprocess.Popen([binary, "-test.run=^TestInteractiveClientTerminal", "-test.v", "-terminal-launch-check"], stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    while chunk := child.stdout.read1(4096):
        os.write(1, chunk)
    os._exit(child.wait())

output = bytearray()
sent = False
finished = False
status = None
try:
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        ready, _, _ = select.select([master], [], [], 0.1)
        if ready:
            try:
                data = os.read(master, 4096)
            except OSError:
                data = b""
            if data:
                output.extend(data)
            if not sent and b"terminal_fixture_ready" in output:
                os.write(master, b"fixture\n")
                sent = True
        child, child_status = os.waitpid(pid, os.WNOHANG)
        if child:
            status = child_status
            finished = True
            break
    if not finished:
        os.killpg(pid, signal.SIGKILL)
        _, status = os.waitpid(pid, 0)
        finished = True
    print(output.decode("utf-8", errors="replace"))
    sys.exit(0 if sent and os.waitstatus_to_exitcode(status) == 0 else 1)
finally:
    if not finished:
        os.killpg(pid, signal.SIGKILL)
        os.waitpid(pid, 0)
    os.close(master)
