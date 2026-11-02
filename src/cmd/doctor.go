package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/pelletier/go-toml/v2"
	"github.com/urfave/cli/v3"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/profile"
)

type doctorSetting struct {
	State string `json:"state"`
	Value *bool  `json:"value,omitempty"`
}

const doctorUnknown = "unknown"

type doctorReport struct {
	SchemaVersion int `json:"schema_version"`
	Build         struct {
		Version   string `json:"version"`
		GoVersion string `json:"go_version"`
		OS        string `json:"os"`
		Arch      string `json:"arch"`
		Revision  string `json:"revision,omitempty"`
		Modified  string `json:"modified,omitempty"`
	} `json:"build"`
	Executable    string         `json:"executable"`
	Profile       profile.Paths  `json:"profile"`
	ProfileStatus profile.Status `json:"profile_status"`
	Terminal      struct {
		Program           string `json:"program"`
		InteractiveInput  bool   `json:"interactive_input"`
		InteractiveOutput bool   `json:"interactive_output"`
		Font              string `json:"font"`
		NerdFontGlyphs    string `json:"nerd_font_glyphs"`
		ImageProtocol     string `json:"image_protocol"`
	} `json:"terminal"`
	Settings        map[string]doctorSetting `json:"settings"`
	Providers       map[string]bool          `json:"providers"`
	SearchProviders map[string]string        `json:"search_providers"`
	Notes           []string                 `json:"notes"`
}

func doctorCommand() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Inspect build, profile, terminal hints and optional provider availability without changing settings",
		Flags: []cli.Flag{&cli.BoolFlag{Name: "json", Usage: "Print machine-readable JSON"}},
		Action: func(_ context.Context, c *cli.Command) error {
			report := collectDoctor(pathsForCommand(c), os.Getenv, exec.LookPath)
			if c.Bool("json") {
				encoder := json.NewEncoder(c.Root().Writer)
				encoder.SetIndent("", "  ")
				return encoder.Encode(report)
			}
			_, err := fmt.Fprintf(
				c.Root().Writer,
				"Xelif %s (%s/%s)\nExecutable: %s\nConfiguration: %s\nData: %s\nState: %s\nTerminal: %s\nFont/glyph support: unknown (not probed)\nUse xelif doctor --json for the complete read-only report.\n",
				report.Build.Version,
				report.Build.OS,
				report.Build.Arch,
				report.Executable,
				report.Profile.ConfigFile,
				report.Profile.DataDir,
				report.Profile.StateDir,
				report.Terminal.Program,
			)
			return err
		},
	}
}

func pathsForCommand(c *cli.Command) profile.Paths {
	paths := profile.ActivePaths()
	if value := c.String("config-file"); value != "" {
		paths.ConfigFile = value
	}
	if value := c.String("hotkey-file"); value != "" {
		paths.HotkeysFile = value
	}
	return paths
}

// collectDoctor reads only allowlisted information. It never starts providers,
// probes a terminal, loads plugins, contacts a network, or dumps environment.
func collectDoctor(
	paths profile.Paths,
	getenv func(string) string,
	lookPath func(string) (string, error),
) doctorReport {
	report := doctorReport{
		SchemaVersion: 1,
		Profile:       paths,
		Settings:      make(map[string]doctorSetting),
		Providers:     make(map[string]bool),
	}
	report.SearchProviders = inspectSearchTools(lookPath)
	report.Build.Version = variable.CurrentVersion + variable.PreReleaseSuffix
	report.Build.GoVersion, report.Build.OS, report.Build.Arch = runtime.Version(), runtime.GOOS, runtime.GOARCH
	populateBuildIdentity(&report)
	report.Executable, _ = os.Executable()
	if report.Executable == "" {
		report.Executable = doctorUnknown
	}
	status, err := profile.Inspect(paths)
	report.ProfileStatus = status
	if err != nil {
		report.Notes = append(report.Notes, "Profile inspection failed; existing files were not changed.")
	}
	populateTerminalReport(&report, getenv)
	populateDoctorSettings(&report, paths, status)
	for _, name := range []string{"rg", "fd", "bat", "exiftool", "zoxide", "rclone"} {
		_, lookupErr := lookPath(name)
		report.Providers[name] = lookupErr == nil
	}
	report.Notes = append(
		report.Notes,
		"Configured values only; omitted configuration keys are reported as unknown, not guessed.",
		"Terminal identity is a hint. Font, glyph coverage and image protocols cannot be verified without an interactive check.",
		"Providers were located on PATH, not executed. No network, credential, plugin, or environment-content inspection was performed.",
	)
	return report
}

func inspectSearchTools(lookPath func(string) (string, error)) map[string]string {
	providers := make(map[string]string)
	for provider, tool := range map[string]string{"spotlight": "/usr/bin/mdfind", "plocate": "plocate", "everything_cli": "es.exe"} {
		_, err := lookPath(tool)
		status := "unavailable"
		if err == nil {
			status = "tool-found-not-executed"
		}
		providers[provider] = status
	}
	if runtime.GOOS == "windows" {
		providers["everything_native"] = "unknown-not-probed"
	} else {
		providers["everything_native"] = "not-applicable"
	}
	return providers
}

func populateBuildIdentity(report *doctorReport) {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, item := range info.Settings {
			switch item.Key {
			case "vcs.revision":
				report.Build.Revision = item.Value
			case "vcs.modified":
				report.Build.Modified = item.Value
			}
		}
	}
}

func populateTerminalReport(report *doctorReport, getenv func(string) string) {
	report.Terminal.Program = doctorUnknown
	if getenv("WT_SESSION") != "" {
		report.Terminal.Program = "windows-terminal"
	} else {
		switch strings.ToLower(getenv("TERM_PROGRAM")) {
		case "wezterm":
			report.Terminal.Program = "wezterm"
		case "iterm.app":
			report.Terminal.Program = "iterm"
		case "apple_terminal":
			report.Terminal.Program = "apple-terminal"
		case "vscode":
			report.Terminal.Program = "vscode"
		case "ghostty":
			report.Terminal.Program = "ghostty"
		}
	}
	report.Terminal.InteractiveInput = isatty.IsTerminal(os.Stdin.Fd())
	report.Terminal.InteractiveOutput = isatty.IsTerminal(os.Stdout.Fd())
	report.Terminal.Font, report.Terminal.NerdFontGlyphs, report.Terminal.ImageProtocol = doctorUnknown, doctorUnknown, doctorUnknown
}

func populateDoctorSettings(report *doctorReport, paths profile.Paths, status profile.Status) {
	var settings struct {
		Nerdfont           *bool `toml:"nerdfont"`
		PlainMode          *bool `toml:"plain_mode"`
		ShowImagePreview   *bool `toml:"show_image_preview"`
		RestoreLastSession *bool `toml:"restore_last_session"`
	}
	if status.ConfigValid {
		if data, readErr := os.ReadFile(paths.ConfigFile); readErr == nil {
			_ = toml.Unmarshal(data, &settings)
		}
	}
	for key, value := range map[string]*bool{
		"nerdfont": settings.Nerdfont, "plain_mode": settings.PlainMode,
		"show_image_preview": settings.ShowImagePreview, "restore_last_session": settings.RestoreLastSession,
	} {
		state := doctorUnknown
		if value != nil {
			state = "configured"
		}
		report.Settings[key] = doctorSetting{State: state, Value: value}
	}
}
