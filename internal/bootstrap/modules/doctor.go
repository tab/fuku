package modules

import (
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/diagnostics"
	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/app/doctor"
	"fuku/internal/app/profiles"
	"fuku/internal/bootstrap/lifecycle"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// Doctor composes the doctor report over the loaded config, which it reports on instead of failing on
func Doctor(cmd *cli.Options, loaded model.Config) fx.Option {
	return fx.Options(
		base,
		standalone(cmd, loaded.Project.Telemetry),
		fx.Supply(loaded),
		fx.Provide(
			newTheme,
			func(theme func() terminal.Theme) cli.Renderer { return newDoctorRenderer(cmd.DoctorFormat, theme) },
			func(c model.Config) model.Project { return c.Project },
			func(identity model.Instance) doctor.Options {
				return doctor.Options{
					Profile:        cmd.Profile,
					ExplicitConfig: cmd.ConfigFile != "",
					Fingerprint:    identity.Fingerprint,
					Version:        buildinfo.Version,
				}
			},
			func(e *diagnostics.Environment) doctor.Environment { return e },
			func(f *diagnostics.Filesystem) doctor.Filesystem { return f },
			func(r *profiles.Resolver) doctor.Profiles { return r },
			func(r *diagnostics.Runtime) doctor.Runtime { return r },
			func(r *doctor.Runner) cli.Checker { return r },
			func(c *cli.Doctor) lifecycle.Command { return c },
		),
		diagnostics.Module,
		doctor.Module,
		instance.Module,
		profiles.Module,
	)
}

// newDoctorRenderer selects the view of the report: the styled report or summary in the theme, or JSON
func newDoctorRenderer(format cli.Format, theme func() terminal.Theme) cli.Renderer {
	switch format {
	case cli.FormatJSON:
		return cli.NewJSON()
	case cli.FormatSummary:
		return tui.NewSummary(theme)
	default:
		return tui.NewReport(theme)
	}
}
