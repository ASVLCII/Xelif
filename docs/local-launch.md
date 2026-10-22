# Launch this Xelif build

Run `Xelif` in PowerShell, Command Prompt, or Git Bash. It opens the dedicated Windows Terminal Xelif profile in the current directory, with the Nerd Font and the configured icon palette. To choose a directory explicitly, run `Xelif "D:\some folder"`.

This machine-local launcher deliberately selects `X:\Xelif\bin\spf.exe`, not the separately installed upstream superfile executable. Rebuilding that binary updates what the command launches.

Launch files:

- `X:\Xelif\bin\Open-Xelif.ps1`: validates the directory and opens the dedicated terminal profile.
- `X:\Xelif\bin\Start-Xelif.ps1`: selects the Xelif runtime directories and starts the exact binary.
- `C:\Users\admin\.local\bin\Xelif.cmd`: PowerShell/Command Prompt entry point in an existing PATH directory.
- `C:\Users\admin\.local\bin\Xelif`: Git Bash entry point.

Runtime settings/state are in `X:\Xelif-work\runtime`, initially copied from the previously approved live profile without deleting or overwriting the original test runtime. The Windows Terminal Xelif profile now uses the stable launcher. Other profiles, user PATH, and the normal upstream superfile configuration were not changed.

`Xelif -Check` reports the binary, starting directory, runtime, and terminal profile without opening a window. Resolution was verified from PowerShell, Command Prompt, and Git Bash. A live launch from a directory containing spaces was verified against the running executable's command line. WSL/Linux shells were not configured.

Launcher correction: all entry points and the Windows Terminal profile use the absolute path to Windows' built-in PowerShell, not `pwsh.exe` or a Codex-bundled shell. Verified Command Prompt and Windows PowerShell with PATH restricted to Windows System32 and the launcher directory, plus a real launch from a directory with spaces. Git Bash resolution was rechecked too. This removes the dependency that failed in the user's existing terminal.
