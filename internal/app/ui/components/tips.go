package components

func tipKey(k string) string  { return HelpKeyStyle.Render(k) }
func tipDesc(d string) string { return HelpDescStyle.Render(d) }

// Tips contains helpful hints displayed in the footer
var Tips = []string{
	tipDesc("stream logs with ") + tipKey("fuku --logs"),
	tipDesc("filter logs by service with ") + tipKey("fuku --logs api"),
	tipDesc("run a specific profile with ") + tipKey("fuku run backend"),
	tipDesc("run without TUI using ") + tipKey("fuku --no-ui"),
	tipDesc("use ") + tipKey("j/k") + tipDesc(" or arrows to navigate"),
	tipDesc("press ") + tipKey("s") + tipDesc(" to stop or start a service"),
	tipDesc("press ") + tipKey("r") + tipDesc(" to restart the selected service"),
	tipDesc("press ") + tipKey("t") + tipDesc(" to hide these tips"),
}
