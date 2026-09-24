# adapters/terminal

The theme, every style, the layout helpers and the service log line formatter. The one place that constructs a `lipgloss.Style`.

`tui`, `cli` and `output` render with what this package builds. They never call `lipgloss.NewStyle()`.

## Theme and styles

`Theme` holds every style that differs between a light and a dark terminal. `NewTheme` builds it for light or dark and never detects.
`bootstrap/modules` builds it from the detected background with `AppearanceSystem.Resolve`. A terminal that does not answer gets dark.
It hands the theme to the TUI, the inline log view and the doctor report. The CLI log view takes no theme.
The TUI rebuilds the theme when the terminal reports its background colour.
The service name palette is part of it. A service name hashes to one colour, so it keeps it for the run.

`styles.go` holds the theme-independent styles as package variables: spacing, panels, text.
`constants.go` holds the widths, timings and glyphs the layout uses.

## Layout

`layout.go` computes the services table columns for a content width, renders a bordered panel as lines and pads or truncates text by display width.
`Blink` is the spring animation behind the loaders. `Tips` are the footer hints.

## Log lines

`Log` formats one service log line: the padded service name in its colour, a separator and the message.
It aligns names to the longest one it has seen. With the JSON log format it writes one JSON object per line instead.
`output.Writer`, `cli.LogView` and `tui.LogView` format through it, so the application log and `fuku logs` look the same.

## Changing it

- a theme-dependent style is a field on `Theme`. A theme-independent one is a `var` in `styles.go`. Nothing else constructs a style
- a style gets a semantic name (`SelectionBgStyle`), never the colour it happens to use
- a layout width is a constant here, not a literal in `tui`
- the audit in `CLAUDE.md` finds a `lipgloss.NewStyle()` outside `theme.go` and `styles.go`
