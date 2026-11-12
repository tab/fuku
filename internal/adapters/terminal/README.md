# adapters/terminal

The theme, every style, the layout helpers and the service log line formatter. It is the one place that constructs a `lipgloss.Style`.
`tui`, `cli` and `output` render with what this package builds.

## Theme and styles

`Theme` holds every style that differs between a light and a dark terminal. `NewTheme` builds it for light or dark and never detects.
`bootstrap/modules` detects the background with `AppearanceSystem.Resolve`, once, the first time a line or a view is styled.
That is why `Log` and the views take the theme as a function. A terminal that does not answer gets dark.
The `fuku mcp` console is fixed dark (see `adapters/mcp`). The TUI rebuilds the theme when the terminal reports its background colour.

The theme holds the service name palette. A service name hashes to one colour, so it keeps that colour for the run.
`styles.go` holds the theme-independent styles as package variables. `constants.go` holds the widths, timings and glyphs.

## Log lines

`Log` formats one service log line: the padded service name in its colour, a separator and the message.
It aligns names to the longest one it has seen. With the JSON log format it writes one JSON object per line.
`output.Writer`, `cli.LogView` and `tui.LogView` format through it, so the application log and `fuku logs` look the same.

## Changing it

- a theme-dependent style is a field on `Theme`. A theme-independent one is a `var` in `styles.go`. Nothing else constructs a style
- a style gets a semantic name (`SelectionBgStyle`), never the colour it uses
- a layout width is a constant here, not a literal in `tui`
