package components

import "strings"

// Tip holds a structured tip with styled and unstyled segments
type Tip struct {
	Segments []TipSegment
}

// TipSegment is a piece of a tip, either a key or description
type TipSegment struct {
	Text  string
	IsKey bool
}

// Render formats the tip using the given theme
func (t Tip) Render(theme Theme) string {
	keyStyle := theme.HelpKeyStyle
	descStyle := theme.HelpDescStyle

	var b strings.Builder

	for _, seg := range t.Segments {
		if seg.IsKey {
			b.WriteString(keyStyle.Render(seg.Text))
		} else {
			b.WriteString(descStyle.Render(seg.Text))
		}
	}

	return b.String()
}

func desc(text string) TipSegment { return TipSegment{Text: text, IsKey: false} }
func key(text string) TipSegment  { return TipSegment{Text: text, IsKey: true} }

// Tips contains helpful hints displayed in the footer
var Tips = []Tip{
	{
		Segments: []TipSegment{
			desc("stream logs with "),
			key("fuku logs"),
		},
	},
	{
		Segments: []TipSegment{
			desc("filter logs by service with "),
			key("fuku logs api"),
		},
	},
	{
		Segments: []TipSegment{
			desc("run a specific profile with "),
			key("fuku run backend"),
		},
	},
	{
		Segments: []TipSegment{
			desc("run without TUI using "),
			key("fuku run --no-ui"),
		},
	},
	{
		Segments: []TipSegment{
			desc("use a custom config with "),
			key("fuku -c path/to/config.yaml"),
		},
	},
	{
		Segments: []TipSegment{
			desc("use "),
			key("j/k"),
			desc(" or arrows to navigate"),
		},
	},
	{
		Segments: []TipSegment{
			desc("press "),
			key("s"),
			desc(" to stop or start a service"),
		},
	},
	{
		Segments: []TipSegment{
			desc("press "),
			key("r"),
			desc(" to restart the selected service"),
		},
	},
	{
		Segments: []TipSegment{
			desc("visit "),
			key("https://getfuku.sh"),
			desc(" for docs and updates"),
		},
	},
	{
		Segments: []TipSegment{
			desc("override config with "),
			key("fuku.override.yaml"),
		},
	},
	{
		Segments: []TipSegment{
			desc("press "),
			key("t"),
			desc(" to hide these tips"),
		},
	},
}
