package navigation

// Navigator provides view switching functionality
type Navigator interface {
	CurrentView() View
	SwitchTo(view View)
	Toggle()
}

type navigator struct {
	current View
}

// NewNavigator creates a new navigator starting with services view
func NewNavigator() Navigator {
	return &navigator{
		current: ViewServices,
	}
}

// CurrentView returns the active view
func (n *navigator) CurrentView() View {
	return n.current
}

// SwitchTo changes to the specified view
func (n *navigator) SwitchTo(view View) {
	n.current = view
}

// Toggle switches between services and logs views
func (n *navigator) Toggle() {
	if n.current == ViewServices {
		n.current = ViewLogs
	} else {
		n.current = ViewServices
	}
}
