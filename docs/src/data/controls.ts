export interface KeyboardControl {
  keys: string[][];
  action: string;
}

export function getKeyboardControls(): KeyboardControl[] {
  return [
    { keys: [["↑", "↓"], ["k", "j"]], action: "Navigate services (or scroll the focused panel)" },
    { keys: [["PgUp", "PgDn"]], action: "Scroll the focused panel" },
    { keys: [["Home", "End"]], action: "Jump to start / end of the focused panel" },
    { keys: [["Enter"]], action: "Open the service info aside (toggles closed when already open)" },
    { keys: [["Tab", "Shift+Tab"]], action: "Cycle aside tabs forward / backward" },
    { keys: [["\\"]], action: "Toggle focus between the services list and the aside" },
    { keys: [["s"]], action: "Stop or start the selected service" },
    { keys: [["r"]], action: "Restart the selected service" },
    { keys: [["ctrl+r"]], action: "Restart all failed services" },
    { keys: [["/"]], action: "Filter services by name" },
    { keys: [["Esc"]], action: "Close the service info aside, or clear filter when it is already closed" },
    { keys: [["q"]], action: "Quit and stop all services" },
  ];
}
