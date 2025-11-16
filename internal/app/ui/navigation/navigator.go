package navigation

// Navigator provides view switching functionality
type Navigator interface {
	// CurrentView returns the active view
	CurrentView() View
	// SwitchTo changes to the specified view
	SwitchTo(view View)
	// Toggle switches between services and logs views
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

func (n *navigator) CurrentView() View {
	return n.current
}

func (n *navigator) SwitchTo(view View) {
	n.current = view
}

func (n *navigator) Toggle() {
	if n.current == ViewServices {
		n.current = ViewLogs
	} else {
		n.current = ViewServices
	}
}
