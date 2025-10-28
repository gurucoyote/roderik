# Legacy WSL-to-Windows Chrome Attachment

These steps document the original workflow for running Roderik inside WSL2 while launching and controlling the Windows Chrome browser. They remain here for historical reference in case the native Windows binary flow is unavailable. You can still rely on them when you need to attach to an existing Windows Chrome profile from WSL2.

1. Run `roderik --desktop <url>` from WSL2. The CLI launches Windows Chrome via `cmd.exe` with `--remote-debugging-port=9222`, `--remote-debugging-address=0.0.0.0`, and a dedicated `WSL2` user-data directory under `%USERPROFILE%\AppData\Local\Google\Chrome\User Data\WSL2`.
2. Ensure the DevTools port is reachable from WSL:
   - Allow inbound TCP on port 9222 through the Windows Firewall, e.g. `New-NetFirewallRule -Direction Inbound -Protocol TCP -LocalPort 9222 -RemoteAddress <WSL_IP>`.
   - If Chrome binds only to `127.0.0.1`, forward the port using `netsh interface portproxy`, SSH port forwarding, or similar tools.
3. Once connected, Roderik attaches to the first existing DevTools page instead of spawning a blank tab, so the visible desktop Chrome window mirrors the CLI session.
4. Navigation hooks reset the active element after each page load; commands such as `search`, `elem`, and `click` keep tracking the desktop browser. When native clicks or typing time out, the CLI falls back to triggering the action in page JavaScript to maintain responsiveness.

These measures are unnecessary when running the native `roderik.exe` on Windows, which is now the recommended flow.

## Desktop-Attach Implementation Notes

The current `--desktop` flag grew from the following plan and is useful background if you need to modify the WSL2→Windows attach flow:

- Detect WSL2 before attempting to launch Windows Chrome. We originally sniffed `/proc/version` for the `Microsoft` string and only engaged the desktop path when both WSL2 and `--desktop` were present.
- Launch Chrome via `cmd.exe /C start "" "<chrome-path>" …` with `--remote-debugging-port=9222`, `--remote-debugging-address=0.0.0.0`, and a dedicated `%USERPROFILE%\AppData\Local\Google\Chrome\User Data\WSL2` profile.
- Wait for the DevTools endpoint to become available (typically by polling `http://<windows-host>:9222/json/version`) before passing the returned WebSocket URL to Rod.
- Skip the headless launcher entirely when a desktop attachment succeeds so the visible Chrome window becomes the controlled session.
- Fall back to the headless profile when either WSL detection fails or DevTools never comes up.

Keeping these guardrails in place prevents us from spawning duplicate listeners or leaking Windows Chrome instances when `roderik` exits unexpectedly.

## Troubleshooting The Windows Chrome Attach

Use these checks when the desktop session fails to connect:

1. **Inspect Chrome’s launch arguments**  
   In the Windows Chrome window visit `chrome://version` and confirm the process has the expected `--remote-debugging-port`, `--remote-debugging-address`, and `--user-data-dir` flags.
2. **Verify the DevTools socket is listening**  
   On Windows run `netstat -an | findstr 9222`. You should see a `LISTENING` row. If not, relaunch Chrome with the correct flags.
3. **Query Windows from WSL**  
   From WSL you can invoke PowerShell to double-check which process owns port 9222:  
   `powershell.exe -Command "Get-NetTCPConnection -LocalPort 9222 | Format-Table LocalAddress,State,OwningProcess"`
4. **Test the connection manually**  
   From WSL curl the DevTools JSON endpoints:  
   `curl -v http://<windows-host>:9222/json/version`  
   `curl -v http://127.0.0.1:9222/json/version`  
   Use whichever address Chrome bound to (`127.0.0.1` or `0.0.0.0`).
5. **Confirm the Windows firewall rules**  
   Ensure the firewall allows the chosen address. For loopback-only bindings, set up `netsh interface portproxy`, `ssh -L`, or `socat` to forward traffic into Windows.

Once these checks pass, restart `roderik --desktop …` and the DevTools handshake should succeed.

## Alternate Chrome MCP Bridge Options

If you prefer to integrate with Chrome through a standalone MCP server instead of the embedded Rod launcher, these community bridges are known to work as of October 2025:

| Project | What it provides | Notes |
| --- | --- | --- |
| **hangwin/mcp-chrome** | Chrome extension + Node bridge exposing 20+ tools | Runs at `http://127.0.0.1:12306/mcp`. Requires installing the extension inside your Windows profile and forwarding the port into WSL2. |
| **ChromeDevTools/chrome-devtools-mcp** | Google-maintained launcher/attach utility | Install via `npx chrome-devtools-mcp@latest`. Supports `--browserUrl` to attach to an existing Chrome instance if DevTools is forwarded. |
| **nicholmikey/chrome-tools-MCP** | DevTools-only bridge with WSL instructions | Launch Chrome with `--remote-debugging-port=9222`, then forward the port (SSH tunnel or `netsh`). Set `CHROME_DEBUG_URL` for the MCP host. |
| **Rainmen-xia/chrome-debug-mcp** | Direct DevTools tunnel (no extension) | Same DevTools forwarding requirements; configure the MCP server with `remote_host=127.0.0.1:9222`. |

Pick the option that best matches your deployment constraints. All of them rely on the same DevTools socket that `roderik --desktop` uses, so the troubleshooting checklist above applies there too.
