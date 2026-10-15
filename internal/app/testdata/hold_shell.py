"""Check the run-command-then-exec-shell lifecycle on a real controlling PTY."""

import errno
import os
import pty
import select
import signal
import subprocess
import sys
import time

shell, command, directory, expected_status, expected_output = sys.argv[1:]
environment = {
    "PATH": "/usr/bin:/bin",
    "TERM": "xterm",
    "ZDOTDIR": directory,
    "PS1": "HOLD_PROMPT> ",
}
pid, terminal = pty.fork()
if pid == 0:
    os.chdir(directory)
    result = subprocess.run([shell, "-fic", command], env=environment)
    os.write(1, f"\nCOMMAND_STATUS:{result.returncode}\n".encode())
    os.execve(shell, [shell, "-f"], environment)

output = b""
sent_input = False
sent_interrupt = False
status = None
try:
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        if select.select([terminal], [], [], 0.05)[0]:
            try:
                chunk = os.read(terminal, 65536)
            except OSError as error:
                if error.errno == errno.EIO:
                    break
                raise
            if not chunk:
                break
            output += chunk
        if b"WAIT_FOR_INTERRUPT" in output and not sent_interrupt:
            os.write(terminal, b"\x03")
            sent_interrupt = True
        if b"HOLD_PROMPT>" in output and not sent_input:
            # The echoed input cannot contain READ_BACK: only execution creates it.
            os.write(terminal, b"printf 'READ_%s\\n' BACK; pwd; exit\n")
            sent_input = True
    done, status = os.waitpid(pid, os.WNOHANG)
    reap_deadline = time.monotonic() + 0.5
    while not done and time.monotonic() < reap_deadline:
        time.sleep(0.01)
        done, status = os.waitpid(pid, os.WNOHANG)
    if not done:
        os.killpg(pid, signal.SIGKILL)
        _, status = os.waitpid(pid, 0)
finally:
    os.close(terminal)

text = output.decode(errors="replace").replace("\r", "")
assert os.waitstatus_to_exitcode(status) == 0, text
assert f"COMMAND_STATUS:{expected_status}\n" in text, text
assert "\nREAD_BACK\n" in text, text
assert directory + "\n" in text, text
assert expected_output in text, text
assert "error on TTY read" not in text, text
