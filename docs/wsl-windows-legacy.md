# Legacy WSL-to-Windows Chrome Attachment

These steps document the original workflow for running Roderik inside WSL2 while launching and controlling the Windows Chrome browser. They remain here for historical reference in case the native Windows binary flow is unavailable.

1. Run `roderik --desktop <url>` from WSL2. The CLI launches Windows Chrome via `cmd.exe` with `--remote-debugging-port=9222`, `--remote-debugging-address=0.0.0.0`, and a dedicated `WSL2` user-data directory under `%USERPROFILE%\AppData\Local\Google\Chrome\User Data\WSL2`.
2. Ensure the DevTools port is reachable from WSL:
   - Allow inbound TCP on port 9222 through the Windows Firewall, e.g. `New-NetFirewallRule -Direction Inbound -Protocol TCP -LocalPort 9222 -RemoteAddress <WSL_IP>`.
   - If Chrome binds only to `127.0.0.1`, forward the port using `netsh interface portproxy`, SSH port forwarding, or similar tools.
3. Once connected, Roderik attaches to the first existing DevTools page instead of spawning a blank tab, so the visible desktop Chrome window mirrors the CLI session.
4. Navigation hooks reset the active element after each page load; commands such as `search`, `elem`, and `click` keep tracking the desktop browser. When native clicks or typing time out, the CLI falls back to triggering the action in page JavaScript to maintain responsiveness.

These measures are unnecessary when running the native `roderik.exe` on Windows, which is now the recommended flow.
